package main

import (
	"encoding/base64"
	"fmt"

	"github.com/libp2p/go-libp2p/core/crypto"
)

func main() {
	for i := 0; i < 10; i++ {
		priv, _, err := crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			panic(err)
		}

		b, err := crypto.MarshalPrivateKey(priv)
		if err != nil {
			panic(err)
		}

		encoded := base64.StdEncoding.EncodeToString(b)
		fmt.Println(encoded)
	}
}
