#!/usr/bin/env python3
"""Rehearse a governance upgrade on a disposable single-validator chain."""
import argparse
import json
from pathlib import Path
import subprocess
import tempfile
import time
import urllib.request


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--old", required=True, type=Path)
    parser.add_argument("--new", required=True, type=Path)
    parser.add_argument("--contract", required=True, type=Path,
                        help="CosmWasm 1.x hackatom.wasm from wasmd v0.50.0")
    parser.add_argument("--work-dir", required=True, type=Path)
    args = parser.parse_args()
    old, new, contract = (str(p.resolve()) for p in (args.old, args.new, args.contract))
    args.work_dir.mkdir(parents=True, exist_ok=True)
    rpc = "http://127.0.0.1:28657"
    chain = "passage-security-local"
    process = None
    with tempfile.TemporaryDirectory(prefix="upgrade-", dir=args.work_dir) as tmp:
        home = Path(tmp)
        log_path = home / "node.log"
        log = log_path.open("w")

        def cli(binary, *cmd, json_output=False):
            command = [binary, *cmd, "--home", tmp]
            if json_output:
                command += ["--output", "json"]
            result = subprocess.run(command, capture_output=True, text=True, timeout=120)
            if result.returncode:
                raise RuntimeError(f"{' '.join(cmd[:3])}: {result.stderr[-2500:]}")
            return json.loads(result.stdout) if json_output else result.stdout.strip()

        def query(binary, *cmd):
            return cli(binary, "query", *cmd, "--node", rpc, json_output=True)

        def height():
            with urllib.request.urlopen(rpc + "/status", timeout=3) as response:
                return int(json.load(response)["result"]["sync_info"]["latest_block_height"])

        def application_height():
            with urllib.request.urlopen(rpc + "/abci_info", timeout=3) as response:
                return int(json.load(response)["result"]["response"]["last_block_height"])

        def wait_until(fn, seconds=90):
            deadline = time.monotonic() + seconds
            while time.monotonic() < deadline:
                try:
                    result = fn()
                    if result:
                        return result
                except (OSError, KeyError, ValueError):
                    pass
                time.sleep(0.5)
            raise RuntimeError("Timed out waiting for local chain")

        def tx(binary, *cmd):
            receipt = cli(binary, "tx", *cmd, "--node", rpc, "--chain-id", chain,
                          "--from", "validator", "--keyring-backend", "test",
                          "--gas", "10000000", "--fees", "1upasg", "-y", json_output=True)
            if int(receipt.get("code", 0)):
                raise RuntimeError(f"CheckTx rejected: {receipt}")
            tx_hash = receipt["txhash"]

            def committed():
                try:
                    return query(binary, "tx", tx_hash)
                except RuntimeError:
                    return None

            result = wait_until(committed)
            if int(result.get("code", 0)):
                raise RuntimeError(f"DeliverTx rejected: {result}")
            return result

        def start(binary):
            return subprocess.Popen([binary, "start", "--home", tmp,
                                     "--minimum-gas-prices", "0upasg",
                                     "--rpc.laddr", "tcp://127.0.0.1:28657",
                                     "--p2p.laddr", "tcp://127.0.0.1:28656",
                                     "--grpc.enable=false"], stdout=log, stderr=log)

        def stop(proc):
            if proc and proc.poll() is None:
                proc.terminate()
                try:
                    proc.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    proc.kill()
                    proc.wait(timeout=10)

        try:
            assert cli(new, "query", "wasm", "libwasmvm-version") == "2.2.9"
            cli(old, "init", "security-rehearsal", "--chain-id", chain, "--default-denom", "upasg")
            cli(old, "keys", "add", "validator", "--keyring-backend", "test")
            cli(old, "keys", "add", "recipient", "--keyring-backend", "test")
            validator = cli(old, "keys", "show", "validator", "-a", "--keyring-backend", "test")
            recipient = cli(old, "keys", "show", "recipient", "-a", "--keyring-backend", "test")
            cli(old, "add-genesis-account", validator, "1000000000000000upasg")
            cli(old, "gentx", "validator", "1000000000000upasg",
                "--chain-id", chain, "--keyring-backend", "test")
            cli(old, "collect-gentxs")
            genesis_path = home / "config" / "genesis.json"
            genesis = json.loads(genesis_path.read_text())
            # v4.0.3's claim InitGenesis hardcodes module account number 0.
            # Reserve it in this disposable genesis so the old binary can boot.
            genesis["app_state"]["auth"]["accounts"][0]["account_number"] = "1"
            gov = genesis["app_state"]["gov"]["params"]
            gov["min_deposit"] = [{"denom": "upasg", "amount": "10"}]
            gov["voting_period"] = "5s"
            gov["expedited_voting_period"] = "2s"
            genesis["consensus"]["params"]["block"]["max_gas"] = "-1"
            genesis_path.write_text(json.dumps(genesis))
            config_path = home / "config" / "config.toml"
            config_path.write_text(config_path.read_text().replace('timeout_commit = "5s"', 'timeout_commit = "1s"'))
            process = start(old)
            wait_until(lambda: height() > 1)

            tx(old, "wasm", "store", contract)
            codes_before = query(old, "wasm", "list-code")
            code_id = codes_before["code_infos"][-1]["code_id"]
            tx(old, "wasm", "instantiate", str(code_id),
               json.dumps({"verifier": validator, "beneficiary": recipient}),
               "--admin", validator, "--label", "security-rehearsal", "--amount", "100upasg")
            contract_address = query(old, "wasm", "list-contract-by-code", str(code_id))["contracts"][0]
            contract_before = query(old, "wasm", "contract", contract_address)
            state_before = query(old, "wasm", "contract-state", "all", contract_address)
            params_before = query(old, "wasm", "params")

            module_account = query(old, "auth", "module-account", "gov")["account"]
            def find_address(value):
                if isinstance(value, dict):
                    if value.get("address"):
                        return value["address"]
                    for nested in value.values():
                        found = find_address(nested)
                        if found:
                            return found
                return None

            authority = find_address(module_account)
            assert authority
            upgrade_height = height() + 30
            proposal = {
                "messages": [{"@type": "/cosmos.upgrade.v1beta1.MsgSoftwareUpgrade",
                              "authority": authority,
                              "plan": {"name": "v4.1.0", "height": str(upgrade_height), "info": ""}}],
                "metadata": "", "deposit": "10upasg", "title": "Security upgrade rehearsal",
                "summary": "Local test of Passage v4.1.0", "expedited": False,
            }
            proposal_path = home / "proposal.json"
            proposal_path.write_text(json.dumps(proposal))
            tx(old, "gov", "submit-proposal", str(proposal_path))
            proposals = query(old, "gov", "proposals")["proposals"]
            proposal_id = max(int(p["id"]) for p in proposals)
            tx(old, "gov", "vote", str(proposal_id), "yes")
            wait_until(lambda: query(old, "upgrade", "plan").get("plan", {}).get("name") == "v4.1.0")
            # CometBFT can stop consensus at an upgrade while its process and
            # RPC remain alive. Detect the upgrade halt, then stop the process.
            wait_until(lambda: 'UPGRADE "v4.1.0" NEEDED' in log_path.read_text(), seconds=90)
            log.flush()
            assert 'UPGRADE "v4.1.0" NEEDED' in log_path.read_text(), "Old node did not halt for the upgrade"
            upgrade_info = json.loads((home / "data" / "upgrade-info.json").read_text())
            assert upgrade_info["name"] == "v4.1.0"
            assert int(upgrade_info["height"]) == upgrade_height
            # RPC's consensus height can include the halted proposal block;
            # the application's committed height must remain one block behind.
            assert application_height() == upgrade_height - 1
            stop(process)

            process = start(new)
            wait_until(lambda: height() > upgrade_height + 2)
            assert query(new, "wasm", "params") == params_before
            assert query(new, "wasm", "list-code") == codes_before
            assert query(new, "wasm", "contract", contract_address) == contract_before
            assert query(new, "wasm", "contract-state", "all", contract_address) == state_before
            tx(new, "wasm", "execute", contract_address, '{"release":{}}')
            tx(new, "bank", "send", validator, recipient, "23upasg")
            balances = query(new, "bank", "balances", recipient)["balances"]
            assert {"denom": "upasg", "amount": "123"} in balances
            applied = query(new, "upgrade", "applied", "v4.1.0")
            assert int(applied["height"]) == upgrade_height
            restart_height = height()
            stop(process)
            process = start(new)
            wait_until(lambda: height() > restart_height + 2)
            print(json.dumps({"result": "PASS", "plan": "v4.1.0", "height": upgrade_height,
                              "runtime_wasmvm": "2.2.9", "contract_state_preserved": True,
                              "contract_execution": "PASS", "bank_send": "PASS",
                              "restart": "PASS"}, indent=2))
        except Exception:
            log.flush()
            print(log_path.read_text()[-4500:])
            raise
        finally:
            stop(process)
            log.close()


if __name__ == "__main__":
    main()
