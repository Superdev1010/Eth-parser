package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

const ethEndpoint = "https://cloudflare-eth.com"

type Transaction struct {
	Hash        string `json:"hash"`
	From        string `json:"from"`
	To          string `json:"to"`
	Value       string `json:"value"`
	BlockNumber string `json:"blockNumber"`
}

type BlockWithTransactions struct {
	Number       string        `json:"number"`
	Transactions []Transaction `json:"transactions"`
}

type RequestPayload struct {
	Jsonrpc string        `json:"jsonrpc"`
	Method  string        `json:"method"`
	Params  []interface{} `json:"params"`
	ID      int           `json:"id"`
}

func sendRPCRequest(method string, params []interface{}) (map[string]interface{}, error) {
	requestPayload := RequestPayload{
		Jsonrpc: "2.0",
		Method:  method,
		Params:  params,
		ID:      1,
	}

	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	resp, err := http.Post(ethEndpoint, "application/json", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	contentType := resp.Header.Get("Content-Type")
	if contentType != "application/json" {
		bodyBytes, _ := io.ReadAll(resp.Body) // Use io.ReadAll instead of ioutil.ReadAll
		return nil, fmt.Errorf("received non-JSON response: %s", string(bodyBytes))
	}

	var responsePayload map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&responsePayload); err != nil {
		return nil, fmt.Errorf("failed to decode JSON response: %v", err)
	}

	return responsePayload, nil
}

func getLatestBlockNumber() (int64, error) {
	response, err := sendRPCRequest("eth_blockNumber", []interface{}{})
	if err != nil {
		return 0, err
	}

	blockHex, ok := response["result"].(string)
	if !ok {
		return 0, fmt.Errorf("invalid response format for block number")
	}

	blockNumber, err := strconv.ParseInt(blockHex[2:], 16, 64)
	if err != nil {
		return 0, err
	}

	return blockNumber, nil
}

func getBlockByNumber(blockNumber string) (*BlockWithTransactions, error) {
	params := []interface{}{blockNumber, true}
	response, err := sendRPCRequest("eth_getBlockByNumber", params)
	if err != nil {
		return nil, err
	}

	resultBytes, err := json.Marshal(response["result"])
	if err != nil {
		return nil, err
	}

	var block BlockWithTransactions
	if err := json.Unmarshal(resultBytes, &block); err != nil {
		return nil, err
	}

	return &block, nil
}

func fetchTransactions(address string, startBlock, endBlock int64) {
	for i := startBlock; i <= endBlock; i++ {
		blockNumberHex := fmt.Sprintf("0x%x", i)

		block, err := getBlockByNumber(blockNumberHex)
		if err != nil {
			log.Printf("Error fetching block %s: %v", blockNumberHex, err)
			continue
		}

		for _, tx := range block.Transactions {
			if tx.From == address || tx.To == address {
				fmt.Printf("Transaction: Block %s | Hash: %s | From: %s | To: %s | Value: %s ETH\n",
					block.Number, tx.Hash, tx.From, tx.To, convertWeiToEther(tx.Value))
			}
		}

		time.Sleep(5 * time.Second)
	}
}

func convertWeiToEther(weiValue string) string {
	wei, _ := strconv.ParseInt(weiValue[2:], 16, 64)
	return fmt.Sprintf("%f", float64(wei)/1e18)
}

func fetchTransactionsHandler(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Query().Get("address")
	startBlockParam := r.URL.Query().Get("startBlock")
	endBlockParam := r.URL.Query().Get("endBlock")

	if address == "" || startBlockParam == "" || endBlockParam == "" {
		http.Error(w, "Please provide address, startBlock, and endBlock parameters", http.StatusBadRequest)
		return
	}

	startBlockRange, err := strconv.ParseInt(startBlockParam, 10, 64)
	if err != nil {
		http.Error(w, "Invalid startBlock parameter", http.StatusBadRequest)
		return
	}

	endBlockRange, err := strconv.ParseInt(endBlockParam, 10, 64)
	if err != nil {
		http.Error(w, "Invalid endBlock parameter", http.StatusBadRequest)
		return
	}

	latestBlock, err := getLatestBlockNumber()
	if err != nil {
		http.Error(w, "Error fetching latest block number: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if endBlockRange > latestBlock {
		endBlockRange = latestBlock
	}

	go fetchTransactions(address, startBlockRange, endBlockRange)

	fmt.Fprintf(w, "Fetching transactions for address: %s from block %d to %d", address, startBlockRange, endBlockRange)
}

func main() {
	http.HandleFunc("/fetch-transactions", fetchTransactionsHandler)
	fmt.Println("Server is running on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil)) // Start the server on port 8080
}
