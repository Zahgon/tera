// Reference vectors recorded from the original crate at its locked version (a sample of the full differential set).
package base64

import (
	"bytes"
	"testing"
)

func TestRustVectors(t *testing.T) {
	cases := []struct {
		s                 string
		data              []byte
		url               bool
		encoded, raw      string
		decoded           []byte
		decodeErr, utfErr string
	}{
		{"", []byte{}, false, "", "", []byte{}, "", ""},
		{"=", []byte{}, false, "", "", []byte{}, "Invalid symbol 61, offset 0.", ""},
		{"==", []byte{}, false, "", "", []byte{}, "Invalid symbol 61, offset 0.", ""},
		{"", []byte{}, false, "", "", []byte{}, "", ""},
		{"=", []byte{}, false, "", "", []byte{}, "Invalid symbol 61, offset 0.", ""},
		{"==", []byte{}, false, "", "", []byte{}, "Invalid symbol 61, offset 0.", ""},
		{"", []byte{}, false, "", "", []byte{}, "", ""},
		{"=", []byte{}, false, "", "", []byte{}, "Invalid symbol 61, offset 0.", ""},
		{"==", []byte{}, false, "", "", []byte{}, "Invalid symbol 61, offset 0.", ""},
		{"", []byte{}, false, "", "", []byte{}, "", ""},
		{"=", []byte{}, false, "", "", []byte{}, "Invalid symbol 61, offset 0.", ""},
		{"==", []byte{}, false, "", "", []byte{}, "Invalid symbol 61, offset 0.", ""},
		{"", []byte{}, false, "", "", []byte{}, "", ""},
		{"=", []byte{}, false, "", "", []byte{}, "Invalid symbol 61, offset 0.", ""},
		{"==", []byte{}, false, "", "", []byte{}, "Invalid symbol 61, offset 0.", ""},
		{"qw", []byte{171}, false, "qw==", "qw", []byte{171}, "", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"qw=", []byte{171}, false, "qw==", "qw", []byte{171}, "", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"qw==", []byte{171}, false, "qw==", "qw", []byte{171}, "", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"Ug", []byte{82}, false, "Ug==", "Ug", []byte{82}, "", ""},
		{"Ug=", []byte{82}, false, "Ug==", "Ug", []byte{82}, "", ""},
		{"Ug==", []byte{82}, false, "Ug==", "Ug", []byte{82}, "", ""},
		{"Kw", []byte{43}, false, "Kw==", "Kw", []byte{43}, "", ""},
		{"Kw=", []byte{43}, false, "Kw==", "Kw", []byte{43}, "", ""},
		{"Kw==", []byte{43}, false, "Kw==", "Kw", []byte{43}, "", ""},
		{"0g", []byte{210}, false, "0g==", "0g", []byte{210}, "", "incomplete utf-8 byte sequence from index 0"},
		{"cg", []byte{114}, false, "cg==", "cg", []byte{114}, "", ""},
		{"!", []byte{165}, false, "pQ==", "pQ", []byte{}, "Invalid symbol 33, offset 0.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"é", []byte{184}, false, "uA==", "uA", []byte{}, "Invalid symbol 195, offset 0.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"jSM==", []byte{141, 35}, false, "jSM=", "jSM", []byte{}, "Invalid symbol 61, offset 3.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"3Ks==", []byte{220, 171}, false, "3Ks=", "3Ks", []byte{}, "Invalid symbol 61, offset 3.", ""},
		{"!AA", []byte{109, 76, 41}, false, "bUwp", "bUwp", []byte{}, "Invalid symbol 33, offset 0.", ""},
		{"AA=A", []byte{232, 141, 169, 246}, false, "6I2p9g==", "6I2p9g", []byte{}, "Invalid symbol 61, offset 2.", "invalid utf-8 sequence of 1 bytes from index 3"},
		{"AAA\nA", []byte{223, 9, 35, 147, 99}, false, "3wkjk2M=", "3wkjk2M", []byte{}, "Invalid symbol 10, offset 3.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"AAAéAA", []byte{26, 187, 69, 227, 166, 68}, false, "GrtF46ZE", "GrtF46ZE", []byte{}, "Invalid symbol 195, offset 3.", "invalid utf-8 sequence of 1 bytes from index 1"},
		{"AAA!AAA", []byte{182, 151, 61, 178, 186, 23, 69}, false, "tpc9sroXRQ==", "tpc9sroXRQ", []byte{}, "Invalid symbol 33, offset 3.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"A=AAAAAA", []byte{112, 176, 214, 43, 211, 161, 125, 232}, false, "cLDWK9Ohfeg=", "cLDWK9Ohfeg", []byte{}, "Invalid symbol 61, offset 1.", "invalid utf-8 sequence of 1 bytes from index 1"},
		{"cmD08tO0pTMV", []byte{114, 96, 244, 242, 211, 180, 165, 51, 21}, false, "cmD08tO0pTMV", "cmD08tO0pTMV", []byte{114, 96, 244, 242, 211, 180, 165, 51, 21}, "", "invalid utf-8 sequence of 1 bytes from index 2"},
		{"AAAAAAA\nA", []byte{64, 58, 94, 156, 18, 43, 217, 171, 175}, false, "QDpenBIr2auv", "QDpenBIr2auv", []byte{}, "Invalid symbol 10, offset 7.", "invalid utf-8 sequence of 1 bytes from index 3"},
		{"AAAéAAAAAA", []byte{232, 36, 23, 120, 217, 87, 37, 90, 150, 76}, false, "6CQXeNlXJVqWTA==", "6CQXeNlXJVqWTA", []byte{}, "Invalid symbol 195, offset 3.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"kHaw/2mNEawonJk==", []byte{144, 118, 176, 255, 105, 141, 17, 172, 40, 156, 153}, false, "kHaw/2mNEawonJk=", "kHaw/2mNEawonJk", []byte{}, "Invalid symbol 61, offset 15.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"AAAAAAAA!AA", []byte{59, 101, 133, 94, 255, 18, 175, 53, 242, 139, 74}, false, "O2WFXv8SrzXyi0o=", "O2WFXv8SrzXyi0o", []byte{}, "Invalid symbol 33, offset 8.", "invalid utf-8 sequence of 1 bytes from index 2"},
		{"AA=AAAAAAAAA", []byte{213, 75, 226, 246, 138, 34, 126, 146, 237, 157, 176, 40}, false, "1Uvi9ooifpLtnbAo", "1Uvi9ooifpLtnbAo", []byte{}, "Invalid symbol 61, offset 2.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"AAAAAAAAAAA=", []byte{246, 82, 183, 1, 27, 235, 188, 31, 226, 75, 174, 167}, false, "9lK3ARvrvB/iS66n", "9lK3ARvrvB/iS66n", []byte{0, 0, 0, 0, 0, 0, 0, 0}, "", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"é", []byte{49}, true, "MQ==", "MQ", []byte{}, "Invalid symbol 195, offset 0.", ""},
		{"uAwO", []byte{184, 12, 14}, true, "uAwO", "uAwO", []byte{184, 12, 14}, "", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"A\nAA", []byte{26, 82, 134, 231}, true, "GlKG5w==", "GlKG5w", []byte{}, "Invalid symbol 10, offset 1.", "invalid utf-8 sequence of 1 bytes from index 2"},
		{"AAéAA", []byte{174, 222, 189, 46, 224}, true, "rt69LuA=", "rt69LuA", []byte{}, "Invalid symbol 195, offset 2.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"AAA!AA", []byte{48, 79, 229, 121, 74, 5}, true, "ME_leUoF", "ME_leUoF", []byte{}, "Invalid symbol 33, offset 3.", "invalid utf-8 sequence of 1 bytes from index 2"},
		{"AA=AAAA", []byte{26, 35, 177, 42, 53, 2, 42}, true, "GiOxKjUCKg==", "GiOxKjUCKg", []byte{}, "Invalid symbol 61, offset 2.", "invalid utf-8 sequence of 1 bytes from index 2"},
		{"\nAAAAAAA", []byte{29, 57, 109, 32, 125, 114, 230, 189}, true, "HTltIH1y5r0=", "HTltIH1y5r0", []byte{}, "Invalid symbol 10, offset 0.", "incomplete utf-8 byte sequence from index 6"},
		{"DBCr0meouh5O", []byte{12, 16, 171, 210, 103, 168, 186, 30, 78}, true, "DBCr0meouh5O", "DBCr0meouh5O", []byte{12, 16, 171, 210, 103, 168, 186, 30, 78}, "", "invalid utf-8 sequence of 1 bytes from index 2"},
		{"AAAAAAéAA", []byte{107, 87, 2, 237, 98, 252, 224, 80, 111}, true, "a1cC7WL84FBv", "a1cC7WL84FBv", []byte{}, "Invalid symbol 195, offset 6.", "invalid utf-8 sequence of 1 bytes from index 3"},
		{"AAA!AAAAAA", []byte{137, 229, 252, 114, 209, 187, 219, 131, 39, 103}, true, "ieX8ctG724MnZw==", "ieX8ctG724MnZw", []byte{}, "Invalid symbol 33, offset 3.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"IwOnI7NXua-sQSk==", []byte{35, 3, 167, 35, 179, 87, 185, 175, 172, 65, 41}, true, "IwOnI7NXua-sQSk=", "IwOnI7NXua-sQSk", []byte{}, "Invalid symbol 61, offset 15.", "invalid utf-8 sequence of 1 bytes from index 2"},
		{"AAAAAAA=AAA", []byte{107, 5, 178, 17, 209, 200, 20, 238, 198, 44, 3}, true, "awWyEdHIFO7GLAM=", "awWyEdHIFO7GLAM", []byte{}, "Invalid symbol 61, offset 7.", "invalid utf-8 sequence of 1 bytes from index 2"},
		{"A\nAAAAAAAAAA", []byte{219, 49, 1, 216, 38, 130, 207, 241, 148, 202, 62, 140}, true, "2zEB2CaCz_GUyj6M", "2zEB2CaCz_GUyj6M", []byte{}, "Invalid symbol 10, offset 1.", "invalid utf-8 sequence of 1 bytes from index 0"},
		{"AAAAAAAAAA\nA", []byte{18, 236, 12, 239, 59, 72, 155, 106, 95, 24, 49, 119}, true, "EuwM7ztIm2pfGDF3", "EuwM7ztIm2pfGDF3", []byte{}, "Invalid symbol 10, offset 10.", "invalid utf-8 sequence of 1 bytes from index 1"},
	}
	for i, c := range cases {
		if got := Encode(c.data, c.url, true); got != c.encoded {
			t.Errorf("case %d padded: %q != %q", i, got, c.encoded)
		}
		if got := Encode(c.data, c.url, false); got != c.raw {
			t.Errorf("case %d raw: %q != %q", i, got, c.raw)
		}
		got, err := Decode(c.s, c.url)
		message := ""
		if err != nil {
			message = err.Error()
		}
		if message != c.decodeErr || err == nil && !bytes.Equal(got, c.decoded) {
			t.Errorf("case %d decode: %v %q, want %v %q", i, got, message, c.decoded, c.decodeErr)
		}
		message = ""
		if err := UTF8Error(c.data); err != nil {
			message = err.Error()
		}
		if message != c.utfErr {
			t.Errorf("case %d utf8: %q != %q", i, message, c.utfErr)
		}
	}
}
