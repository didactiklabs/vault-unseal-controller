package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/hashicorp/vault/api"
)

const dotCharacter = 46

func isHiddenFile(file string) bool {
	return file[0] == dotCharacter
}

// getVaultShards return a map with threshold keys
func getVaultShards(secretPath string) (shards []string, err error) {
	files, _ := ioutil.ReadDir(secretPath)
	for _, file := range files {
		filePath := secretPath + "/" + file.Name()
		if !isHiddenFile(file.Name()) {
			content, err := ioutil.ReadFile(filePath)
			if err != nil {
				log.Fatalf("unable to read content of '%s' file: %v\n", file.Name(), err)
				return nil, err
			}
			shards = append(shards, string(content))
		}
	}
	return shards, nil
}

func main() {
	_, addrIsSet := os.LookupEnv("VAULT_ADDR")
	if !addrIsSet {
		log.Fatalln("VAULT_ADDR env var should not be empty !")
	}
	_, secretIsSet := os.LookupEnv("UNSEALER_SECRET_PATH")
	if !secretIsSet {
		log.Fatalln("UNSEALER_SECRET_PATH env var should not be empty !")
	}

	nodeAddr := os.Getenv("VAULT_ADDR")

	// Configure Client
	config := api.DefaultConfig()
	client, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("unable to initialize Vault client: %v\n", err)
	}

	// Do Unseal for each threshold key
	shards, err := getVaultShards(os.Getenv("UNSEALER_SECRET_PATH"))
	if err != nil {
		log.Fatalf("unable to get unseal secret path: %v\n", err)
	}
	for _, keyShard := range shards {
		r, err := client.Sys().Unseal(keyShard)
		if err != nil {
			log.Fatalf("unable to unseal %v instance: %v\n", nodeAddr, err)
		}
		log.Printf("Vault response per shard: %v\n", r)
	}
	log.Printf("Vault instance '%s' is now unsealed !\n", nodeAddr)
}
