package contrib

import "github.com/Keats/tera/tera"

// JSONEncode encodes a value as JSON. It takes an optional `pretty` bool argument (false by default).
//
//	{{ value | json_encode }}
//	{{ value | json_encode(pretty=true) }}
var JSONEncode = tera.TypedFilter(tera.ArgValueRef, func(val tera.Value, kwargs tera.Kwargs, _ *tera.State) (tera.Value, error) {
	pretty, _, err := tera.KwargsGet(kwargs, "pretty", tera.ArgBool)
	if err != nil {
		return tera.Undefined(), err
	}
	out, err := tera.MarshalJSON(val, pretty)
	if err != nil {
		return tera.Undefined(), tera.ErrorMessage("Error encoding to JSON: " + err.Error())
	}
	return tera.NormalString(out), nil
})
