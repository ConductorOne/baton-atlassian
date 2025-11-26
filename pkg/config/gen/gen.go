package main

import (
	cfg "github.com/conductorone/baton-atlassian/pkg/config"
	"github.com/conductorone/baton-sdk/pkg/config"
)

func main() {
	config.Generate("atlassian", cfg.Configuration)
}
