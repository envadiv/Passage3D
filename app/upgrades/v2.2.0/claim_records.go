package v2

import (
	"encoding/csv"
	"errors"
	"log"
	"os"

	"github.com/cosmos/cosmos-sdk/types"
	claimtypes "github.com/envadiv/Passage3D/x/claim/types"
)

func readCsvFile(filePath string) ([][]string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Fatal("Unable to read input file "+filePath, err)
	}
	defer f.Close()

	csvReader := csv.NewReader(f)
	records, err := csvReader.ReadAll()
	if err != nil {
		log.Fatal("Unable to parse file as CSV for "+filePath, err)
	}

	return records, nil
}

func parseRecords(records [][]string) ([]claimtypes.ClaimRecord, error) {
	claimRecords := []claimtypes.ClaimRecord{}
	for _, record := range records {
		amount, ok := types.NewIntFromString(record[1])
		if !ok {
			return []claimtypes.ClaimRecord{}, errors.New("error while reading balances")
		}
		claimRecords = append(claimRecords, claimtypes.ClaimRecord{
			Address:         record[0],
			ClaimableAmount: types.NewCoins(types.NewCoin("upasg", amount)),
			ActionCompleted: []bool{false},
		})
	}

	return claimRecords, nil
}

func loadCsv() []claimtypes.ClaimRecord {
	files := []string{"../app/upgrades/v2.2.0/ss1.csv", "../app/upgrades/v2.2.0/ss2.csv"}

	var claimRecords []claimtypes.ClaimRecord
	for _, file := range files {
		data, err := readCsvFile(file)
		if err != nil {
			panic(err)
		}

		records, err := parseRecords(data)
		if err != nil {
			panic(err)
		}
		claimRecords = append(claimRecords, records...)
	}

	return claimRecords
}
