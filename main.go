package main

import (
	loop "github.com/CameronVoglerOlive/LookupNPI/loop"
)

func main() {
	if err := loop.Serve(); err != nil {
		panic(err)
	}
}
