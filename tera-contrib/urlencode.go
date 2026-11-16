package contrib

import (
	"github.com/Keats/tera/tera"
	"github.com/Keats/tera/tera-contrib/internal/percent"
)

// fragmentEncodeSet is the WHATWG fragment percent-encode set.
var fragmentEncodeSet = percent.CONTROLS.Add(' ').Add('"').Add('<').Add('>').Add('`')

// pathEncodeSet is the WHATWG path percent-encode set.
var pathEncodeSet = fragmentEncodeSet.Add('#').Add('?').Add('{').Add('}')

// userinfoEncodeSet is the WHATWG userinfo percent-encode set.
var userinfoEncodeSet = pathEncodeSet.Add('/').Add(':').Add(';').Add('=').Add('@').Add('[').Add('\\').Add(']').
	Add('^').Add('|')

// pythonEncodeSet matches Python's urllib.parse.quote with `/` left unescaped.
var pythonEncodeSet = userinfoEncodeSet.Remove('/').Add('%').Add(':').Add('?').Add('#').Add('[').Add(']').Add('@').
	Add('!').Add('$').Add('&').Add('\'').Add('(').Add(')').Add('*').Add('+').Add(',').Add(';').Add('=')

// URLEncode percent-encodes reserved URI characters like Python's urllib.parse.quote, leaving `/` as is.
//
//	{{ value | urlencode }}
var URLEncode = tera.TypedFilter(tera.ArgStr, func(val string, _ tera.Kwargs, _ *tera.State) (tera.Value, error) {
	return tera.NormalString(percent.Encode(val, pythonEncodeSet)), nil
})

// URLEncodeStrict percent-encodes every character that is not ASCII alphanumeric, `/` included.
//
//	{{ value | urlencode_strict }}
var URLEncodeStrict = tera.TypedFilter(tera.ArgStr, func(val string, _ tera.Kwargs, _ *tera.State) (tera.Value, error) {
	return tera.NormalString(percent.Encode(val, percent.NON_ALPHANUMERIC)), nil
})
