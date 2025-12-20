package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/rserag/terraform-provider-nameam/internal/provider"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "enable debug mode")
	flag.Parse()

	err := providerserver.Serve(
		context.Background(),
		provider.New,
		providerserver.ServeOpts{
			Address: "registry.terraform.io/rserag/nameam",
			Debug:   debug,
		},
	)
	if err != nil {
		log.Fatal(err)
	}
}
