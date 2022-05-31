//go:generate go run github.com/99designs/gqlgen

package graphql

type Resolver struct{}

// TODO cache repeated db calls to improve efficiency like stat.PoolStakesLookup, UnstakeLookup etc
