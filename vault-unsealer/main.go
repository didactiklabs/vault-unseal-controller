package main

import (
	"log"
	"os"
	"strings"

	"github.com/hashicorp/vault/api"
)

const dotCharacter = 46

func isHiddenFile(file string) bool {
	return file[0] == dotCharacter
}

// getVaultShards return a map with unseal keys
func getVaultShards(secretPath string, keyPrefix string) (shards []string, err error) {
	files, _ := os.ReadDir(secretPath)
	for _, file := range files {
		fileName := file.Name()
		// Skip hidden files
		if isHiddenFile(fileName) {
			continue
		}
		// If keyPrefix is set, only include files that start with the prefix
		if keyPrefix != "" && !strings.HasPrefix(fileName, keyPrefix) {
			log.Printf("Skipping file '%s' (does not match prefix '%s')\n", fileName, keyPrefix)
			continue
		}

		filePath := secretPath + "/" + fileName
		content, err := os.ReadFile(filePath)
		if err != nil {
			log.Fatalf("unable to read content of '%s' file: %v\n", fileName, err)
			return nil, err
		}
		log.Printf("Using unseal key from file: %s\n", fileName)
		shards = append(shards, string(content))
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
	keyPrefix := os.Getenv("UNSEALER_KEY_PREFIX") // Optional: filter keys by prefix

	// Configure Client
	config := api.DefaultConfig()
	client, err := api.NewClient(config)
	if err != nil {
		log.Fatalf("unable to initialize Vault client: %v\n", err)
	}

	// Do Unseal for each unseal key
	shards, err := getVaultShards(os.Getenv("UNSEALER_SECRET_PATH"), keyPrefix)
	if err != nil {
		log.Fatalf("unable to get unseal secret path: %v\n", err)
	}

	if len(shards) == 0 {
		log.Fatalf("no unseal keys found in secret path (prefix: '%s')\n", keyPrefix)
	}

	log.Printf("Found %d unseal key(s) to use\n", len(shards))

	for _, keyShard := range shards {
		r, err := client.Sys().Unseal(keyShard)
		if err != nil {
			log.Fatalf("unable to unseal %v instance: %v\n", nodeAddr, err)
		}
		log.Printf("Vault response per shard: %v\n", r)
	}
	log.Printf("Vault instance '%s' is now unsealed !\n", nodeAddr)
}
