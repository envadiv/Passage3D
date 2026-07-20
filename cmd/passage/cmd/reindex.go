package cmd

// Exposes CometBFT's `reindex-event` tooling under `passage comet
// reindex-event`. Adapted from cometbft v0.38.23
// cmd/cometbft/commands/reindex_event.go (Apache-2.0) with three deliberate
// changes requested by Passage validators post-v4.0.2:
//
//  1. Config comes from the SDK server context (so --home resolves to the
//     passage home, not cometbft's default) instead of the cometbft
//     package-level config that is never populated inside an SDK CLI.
//  2. RunE instead of Run: every failure path returns an error, so the
//     process exits non-zero — upstream printed the error and exited 0,
//     which breaks scripting.
//  3. A final start/end summary line after the progress bar.
//
// Re-running overlapping ranges is safe: the kv indexer upserts by
// height/tx-hash, so re-indexed heights simply overwrite the same keys.
// Offline tool — stop the node before running it.

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	dbm "github.com/cometbft/cometbft-db"

	abcitypes "github.com/cometbft/cometbft/abci/types"
	cmtcfg "github.com/cometbft/cometbft/config"
	cmtos "github.com/cometbft/cometbft/libs/os"
	"github.com/cometbft/cometbft/libs/progressbar"
	"github.com/cometbft/cometbft/state"
	"github.com/cometbft/cometbft/state/indexer"
	blockidxkv "github.com/cometbft/cometbft/state/indexer/block/kv"
	"github.com/cometbft/cometbft/state/indexer/sink/psql"
	"github.com/cometbft/cometbft/state/txindex"
	"github.com/cometbft/cometbft/state/txindex/kv"
	"github.com/cometbft/cometbft/store"
	"github.com/cometbft/cometbft/types"

	"github.com/cosmos/cosmos-sdk/server"
)

var (
	errReindexHeightNotAvailable = errors.New("height is not available")
	errReindexInvalidRequest     = errors.New("invalid request")
)

// NewReindexEventCmd constructs `passage comet reindex-event`, preserving the
// upstream name, alias, and --start-height/--end-height semantics.
func NewReindexEventCmd() *cobra.Command {
	var startHeight, endHeight int64

	cmd := &cobra.Command{
		Use:     "reindex-event",
		Aliases: []string{"reindex_event"},
		Short:   "reindex events to the event store backends",
		Long: `
reindex-event is an offline tool to re-index block and tx events to the
eventsinks. Run it when the event store backend was dropped/disconnected or
you want to replace the backend. The default start-height is 0, meaning the
tool starts from the blockstore base height (inclusive); the default
end-height is 0, meaning it reindexes to the latest block height (inclusive).

Note: requires stored ABCI Responses — do not set discard_abci_responses to
true if you want to use this command. Stop the node before running it.
Re-running overlapping ranges is safe (same keys are overwritten).
`,
		Example: `
	passage comet reindex-event
	passage comet reindex-event --start-height 2
	passage comet reindex-event --end-height 10
	passage comet reindex-event --start-height 2 --end-height 10
`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			serverCtx := server.GetServerContextFromCmd(cmd)
			cfg := serverCtx.Config

			bs, ss, err := loadStateAndBlockStoreForReindex(cfg)
			if err != nil {
				return fmt.Errorf("event re-index failed: %w", err)
			}
			defer func() {
				_ = bs.Close()
				_ = ss.Close()
			}()

			st, err := ss.Load()
			if err != nil {
				return fmt.Errorf("event re-index failed: %w", err)
			}

			start, end, err := checkValidReindexHeight(bs, startHeight, endHeight)
			if err != nil {
				return fmt.Errorf("event re-index failed: %w", err)
			}

			bi, ti, err := loadReindexEventSinks(cfg, st.ChainID)
			if err != nil {
				return fmt.Errorf("event re-index failed: %w", err)
			}

			if err := reindexEvents(cmd, reindexArgs{
				startHeight:  start,
				endHeight:    end,
				blockIndexer: bi,
				txIndexer:    ti,
				blockStore:   bs,
				stateStore:   ss,
			}); err != nil {
				return fmt.Errorf("event re-index failed: %w", err)
			}

			fmt.Printf("event re-index finished: start_height=%d end_height=%d blocks=%d\n",
				start, end, end-start+1)
			return nil
		},
	}

	cmd.Flags().Int64Var(&startHeight, "start-height", 0, "the block height to start re-indexing from (0 = blockstore base)")
	cmd.Flags().Int64Var(&endHeight, "end-height", 0, "the block height to finish re-indexing at (0 = latest)")
	return cmd
}

func loadStateAndBlockStoreForReindex(cfg *cmtcfg.Config) (*store.BlockStore, state.Store, error) {
	dbType := dbm.BackendType(cfg.DBBackend)

	if !cmtos.FileExists(filepath.Join(cfg.DBDir(), "blockstore.db")) {
		return nil, nil, fmt.Errorf("no blockstore found in %v", cfg.DBDir())
	}
	blockStoreDB, err := dbm.NewDB("blockstore", dbType, cfg.DBDir())
	if err != nil {
		return nil, nil, err
	}
	blockStore := store.NewBlockStore(blockStoreDB)

	if !cmtos.FileExists(filepath.Join(cfg.DBDir(), "state.db")) {
		return nil, nil, fmt.Errorf("no statestore found in %v", cfg.DBDir())
	}
	stateDB, err := dbm.NewDB("state", dbType, cfg.DBDir())
	if err != nil {
		return nil, nil, err
	}
	stateStore := state.NewStore(stateDB, state.StoreOptions{
		DiscardABCIResponses: cfg.Storage.DiscardABCIResponses,
	})

	return blockStore, stateStore, nil
}

func loadReindexEventSinks(cfg *cmtcfg.Config, chainID string) (indexer.BlockIndexer, txindex.TxIndexer, error) {
	switch strings.ToLower(cfg.TxIndex.Indexer) {
	case "null":
		return nil, nil, errors.New("found null event sink, please check the tx-index section in the config.toml")
	case "psql":
		conn := cfg.TxIndex.PsqlConn
		if conn == "" {
			return nil, nil, errors.New("the psql connection settings cannot be empty")
		}
		es, err := psql.NewEventSink(conn, chainID)
		if err != nil {
			return nil, nil, err
		}
		return es.BlockIndexer(), es.TxIndexer(), nil
	case "kv":
		st, err := dbm.NewDB("tx_index", dbm.BackendType(cfg.DBBackend), cfg.DBDir())
		if err != nil {
			return nil, nil, err
		}
		txIndexer := kv.NewTxIndex(st)
		blockIndexer := blockidxkv.New(dbm.NewPrefixDB(st, []byte("block_events")))
		return blockIndexer, txIndexer, nil
	default:
		return nil, nil, fmt.Errorf("unsupported event sink type: %s", cfg.TxIndex.Indexer)
	}
}

type reindexArgs struct {
	startHeight  int64
	endHeight    int64
	blockIndexer indexer.BlockIndexer
	txIndexer    txindex.TxIndexer
	blockStore   state.BlockStore
	stateStore   state.Store
}

func reindexEvents(cmd *cobra.Command, args reindexArgs) error {
	var bar progressbar.Bar
	bar.NewOption(args.startHeight-1, args.endHeight)

	fmt.Println("start re-indexing events:")
	defer bar.Finish()
	for height := args.startHeight; height <= args.endHeight; height++ {
		select {
		case <-cmd.Context().Done():
			return fmt.Errorf("event re-index terminated at height %d: %w", height, cmd.Context().Err())
		default:
			block := args.blockStore.LoadBlock(height)
			if block == nil {
				return fmt.Errorf("not able to load block at height %d from the blockstore", height)
			}

			resp, err := args.stateStore.LoadFinalizeBlockResponse(height)
			if err != nil {
				return fmt.Errorf("not able to load ABCI Response at height %d from the statestore", height)
			}

			e := types.EventDataNewBlockEvents{
				Height: height,
				Events: resp.Events,
			}

			numTxs := len(resp.TxResults)
			if numTxs > 0 {
				batch := txindex.NewBatch(int64(numTxs))
				for idx, txResult := range resp.TxResults {
					tr := abcitypes.TxResult{
						Height: height,
						Index:  uint32(idx),
						Tx:     block.Txs[idx],
						Result: *txResult,
					}
					if err = batch.Add(&tr); err != nil {
						return fmt.Errorf("adding tx to batch: %w", err)
					}
				}
				if err := args.txIndexer.AddBatch(batch); err != nil {
					return fmt.Errorf("tx event re-index at height %d failed: %w", height, err)
				}
			}

			if err := args.blockIndexer.Index(e); err != nil {
				return fmt.Errorf("block event re-index at height %d failed: %w", height, err)
			}
		}

		bar.Play(height)
	}

	return nil
}

func checkValidReindexHeight(bs state.BlockStore, startHeight, endHeight int64) (int64, int64, error) {
	base := bs.Base()

	if startHeight == 0 {
		startHeight = base
		fmt.Printf("set the start block height to the base height of the blockstore %d\n", base)
	}
	if startHeight < base {
		return 0, 0, fmt.Errorf("%w (requested start height: %d, base height: %d)",
			errReindexHeightNotAvailable, startHeight, base)
	}

	height := bs.Height()
	if startHeight > height {
		return 0, 0, fmt.Errorf("%w (requested start height: %d, store height: %d)",
			errReindexHeightNotAvailable, startHeight, height)
	}
	if endHeight == 0 || endHeight > height {
		endHeight = height
		fmt.Printf("set the end block height to the latest height of the blockstore %d\n", height)
	}
	if endHeight < base {
		return 0, 0, fmt.Errorf("%w (requested end height: %d, base height: %d)",
			errReindexHeightNotAvailable, endHeight, base)
	}
	if endHeight < startHeight {
		return 0, 0, fmt.Errorf("%w (requested the end height: %d is less than the start height: %d)",
			errReindexInvalidRequest, endHeight, startHeight)
	}

	return startHeight, endHeight, nil
}
