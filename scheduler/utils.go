package main

import (
	"fmt"
	"math/rand"
)

func UUID() string {
	return fmt.Sprintf("%06d", int(rand.Float64()*999999))
}
