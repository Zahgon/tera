package contrib

import (
	"fmt"

	"github.com/Keats/tera/tera"
	"github.com/Keats/tera/tera-contrib/internal/rand"
)

// GetRandom returns a random integer between `start` (inclusive) and `end` (exclusive). With a `seed` string the
// result is reproducible.
//
//	{{ get_random(start=1, end=100) }}
//	{{ get_random(start=1, end=100, seed="page") }}
var GetRandom = tera.FunctionFunc(func(kwargs tera.Kwargs, _ *tera.State) (tera.Value, error) {
	start, err := tera.KwargsMustGet(kwargs, "start", tera.ArgI64)
	if err != nil {
		return tera.Undefined(), err
	}
	end, err := tera.KwargsMustGet(kwargs, "end", tera.ArgI64)
	if err != nil {
		return tera.Undefined(), err
	}
	if start >= end {
		return tera.Undefined(), tera.ErrorMessage(fmt.Sprintf(
			"get_random: `start` (%d) must be less than `end` (%d).", start, end))
	}
	seed, ok, err := tera.KwargsGet(kwargs, "seed", tera.ArgString)
	if err != nil {
		return tera.Undefined(), err
	}
	if ok {
		return tera.I64(rand.SeedFromU64(rand.SeedFromString(seed)).RandomRangeI64(start, end)), nil
	}
	return tera.I64(rand.RandomRangeI64(start, end)), nil
})

// Shuffle shuffles an array. With a `seed` string the order is reproducible.
//
//	{{ list | shuffle }}
//	{{ list | shuffle(seed="page") }}
var Shuffle = tera.TypedFilter(tera.ArgVec(tera.ArgValue), func(val []tera.Value, kwargs tera.Kwargs, _ *tera.State) (tera.Value, error) {
	if len(val) == 0 {
		return tera.Array(val), nil
	}
	seed, ok, err := tera.KwargsGet(kwargs, "seed", tera.ArgString)
	if err != nil {
		return tera.Undefined(), err
	}
	swap := func(i, j int) { val[i], val[j] = val[j], val[i] }
	if ok {
		rand.SeedFromU64(rand.SeedFromString(seed)).Shuffle(len(val), swap)
	} else {
		rand.Shuffle(len(val), swap)
	}
	return tera.Array(val), nil
})
