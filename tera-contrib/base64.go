package contrib

import (
	"github.com/Keats/tera/tera"
	"github.com/Keats/tera/tera-contrib/internal/base64"
)

// B64Encode encodes a string to base64. It takes an optional `url_safe` bool argument (false by default) to use
// the URL-safe alphabet and a `padded` bool argument (true by default) to add padding.
//
//	{{ value | b64_encode }}
//	{{ value | b64_encode(url_safe=true, padded=false) }}
var B64Encode = tera.TypedFilter(tera.ArgStr, func(val string, kwargs tera.Kwargs, _ *tera.State) (tera.Value, error) {
	urlSafe, _, err := tera.KwargsGet(kwargs, "url_safe", tera.ArgBool)
	if err != nil {
		return tera.Undefined(), err
	}
	padded, ok, err := tera.KwargsGet(kwargs, "padded", tera.ArgBool)
	if err != nil {
		return tera.Undefined(), err
	}
	if !ok {
		padded = true
	}
	return tera.NormalString(base64.Encode([]byte(val), urlSafe, padded)), nil
})

// B64Decode decodes a base64 string, accepting input with or without padding. It takes an optional `url_safe`
// bool argument to use the URL-safe alphabet. The decoded bytes must be valid UTF-8.
//
//	{{ value | b64_decode }}
//	{{ value | b64_decode(url_safe=true) }}
var B64Decode = tera.TypedFilter(tera.ArgStr, func(val string, kwargs tera.Kwargs, _ *tera.State) (tera.Value, error) {
	urlSafe, _, err := tera.KwargsGet(kwargs, "url_safe", tera.ArgBool)
	if err != nil {
		return tera.Undefined(), err
	}
	decoded, err := base64.Decode(val, urlSafe)
	if err != nil {
		return tera.Undefined(), tera.ErrorMessage("Invalid base64: " + err.Error())
	}
	if err := base64.UTF8Error(decoded); err != nil {
		return tera.Undefined(), tera.ErrorMessage("Invalid UTF-8: " + err.Error())
	}
	return tera.NormalString(string(decoded)), nil
})
