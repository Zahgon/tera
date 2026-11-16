// Reference vectors recorded from the original crate at its locked version (a sample of the full differential set).
package percent

import (
	"testing"
)

func TestRustVectors(t *testing.T) {
	cases := []struct {
		s        string
		controls bool
		want     string
	}{
		{"\u0000\u0001\u0002\u0003\u0004\u0005\u0006\u0007\b\t\n\u000b\f\r\u000e\u000f\u0010\u0011\u0012\u0013\u0014\u0015\u0016\u0017\u0018\u0019\u001a\u001b\u001c\u001d\u001e\u001f !\"#$%&'()*+,-./0123456789:;<=>?@ABCDEFGHIJKLMNOPQRSTUVWXYZ[\\]^_`abcdefghijklmnopqrstuvwxyz{|}~", false, "%00%01%02%03%04%05%06%07%08%09%0A%0B%0C%0D%0E%0F%10%11%12%13%14%15%16%17%18%19%1A%1B%1C%1D%1E%1F%20%21%22%23%24%25%26%27%28%29%2A%2B%2C%2D%2E%2F0123456789%3A%3B%3C%3D%3E%3F%40ABCDEFGHIJKLMNOPQRSTUVWXYZ%5B%5C%5D%5E%5F%60abcdefghijklmnopqrstuvwxyz%7B%7C%7D%7E%7F"},
		{"リ谰噌埏", false, "%E3%83%AA%E8%B0%B0%E5%99%8C%E5%9F%8F"},
		{"", false, ""},
		{"㰟᭧䄭롒鲜遽᎜", false, "%E3%B0%9F%E1%AD%A7%E4%84%AD%EB%A1%92%E9%B2%9C%E9%81%BD%E1%8E%9C"},
		{"맼톫⫢혾엾৙䞣挳", false, "%EB%A7%BC%ED%86%AB%E2%AB%A2%ED%98%BE%EC%97%BE%E0%A7%99%E4%9E%A3%E6%8C%B3"},
		{"峂ⅴ晻韦Ⱞ彞㏝㾥鱵", false, "%E5%B3%82%E2%85%B4%E6%99%BB%E9%9F%A6%E2%B0%AE%E5%BD%9E%E3%8F%9D%E3%BE%A5%E9%B1%B5"},
		{"뫦ꤴⒾ퓑ᰩ윉䎎竡ꙫ칔尾", false, "%EB%AB%A6%EA%A4%B4%E2%92%BE%ED%93%91%E1%B0%A9%EC%9C%89%E4%8E%8E%E7%AB%A1%EA%99%AB%EC%B9%94%E5%B0%BE"},
		{"闢顺㔚í闍紕㤼", false, "%E9%97%A2%E9%A1%BA%E3%94%9A%C3%AD%E9%97%8D%E7%B4%95%E3%A4%BC"},
		{"ː臜鼷ნῊ⪣繚", false, "%CB%90%E8%87%9C%E9%BC%B7%E1%83%9C%E1%BF%8A%E2%AA%A3%E7%B9%9A"},
		{"띲", false, "%EB%9D%B2"},
		{"쇧叙愑", false, "%EC%87%A7%E5%8F%99%E6%84%91"},
		{"玍ࠀ냌䴩댈ᬜ禉櫚ꗯ猨祼", false, "%E7%8E%8D%E0%A0%80%EB%83%8C%E4%B4%A9%EB%8C%88%E1%AC%9C%E7%A6%89%E6%AB%9A%EA%97%AF%E7%8C%A8%E7%A5%BC"},
		{"盓庪Ẳ", false, "%E7%9B%93%E5%BA%AA%E1%BA%B2"},
		{"ṣ䀻䅱", false, "%E1%B9%A3%E4%80%BB%E4%85%B1"},
		{"䊀鷞枓䢢䋡ꔪľ㾉쪺", false, "%E4%8A%80%E9%B7%9E%E6%9E%93%E4%A2%A2%E4%8B%A1%EA%94%AA%C4%BE%E3%BE%89%EC%AA%BA"},
		{"", false, ""},
		{"ꩇ俎皑謃ⲉ輩฼⑭诵磜氥", false, "%EA%A9%87%E4%BF%8E%E7%9A%91%E8%AC%83%E2%B2%89%E8%BC%A9%E0%B8%BC%E2%91%AD%E8%AF%B5%E7%A3%9C%E6%B0%A5"},
		{"鄝䗝", false, "%E9%84%9D%E4%97%9D"},
		{"芾壾댗", false, "%E8%8A%BE%E5%A3%BE%EB%8C%97"},
		{"", false, ""},
		{"套⑕ڝ腄䄺⢌", false, "%E5%A5%97%E2%91%95%DA%9D%E8%85%84%E4%84%BA%E2%A2%8C"},
		{"耄౉", false, "%E8%80%84%E0%B1%89"},
		{"橦쬮⋈೪艏ꯙ砚꡺齉", false, "%E6%A9%A6%EC%AC%AE%E2%8B%88%E0%B3%AA%E8%89%8F%EA%AF%99%E7%A0%9A%EA%A1%BA%E9%BD%89"},
		{"䁡藨룧䥺懫", false, "%E4%81%A1%E8%97%A8%EB%A3%A7%E4%A5%BA%E6%87%AB"},
		{"Ⓛ㰽邦ꕅ슥쐋꽈뺝⍄꯾媑", false, "%E2%93%81%E3%B0%BD%E9%82%A6%EA%95%85%EC%8A%A5%EC%90%8B%EA%BD%88%EB%BA%9D%E2%8D%84%EA%AF%BE%E5%AA%91"},
		{"聁쇕浈겜憂᩽쳰඲", false, "%E8%81%81%EC%87%95%E6%B5%88%EA%B2%9C%E6%86%82%E1%A9%BD%EC%B3%B0%E0%B6%B2"},
		{"㥅眊撼穼呏⩔橦", false, "%E3%A5%85%E7%9C%8A%E6%92%BC%E7%A9%BC%E5%91%8F%E2%A9%94%E6%A9%A6"},
		{"Ể鰃셵ጘ赒뷮", false, "%E1%BB%82%E9%B0%83%EC%85%B5%E1%8C%98%E8%B5%92%EB%B7%AE"},
		{"倖ꢞ", false, "%E5%80%96%EA%A2%9E"},
		{"摋縥쪪螘胢鸇⊡౥에", false, "%E6%91%8B%E7%B8%A5%EC%AA%AA%E8%9E%98%E8%83%A2%E9%B8%87%E2%8A%A1%E0%B1%A5%EC%97%90"},
		{"緻⏏娐⣸䡎앪郔䪽", false, "%E7%B7%BB%E2%8F%8F%E5%A8%90%E2%A3%B8%E4%A1%8E%EC%95%AA%E9%83%94%E4%AA%BD"},
		{"얮", false, "%EC%96%AE"},
		{"㺃뙦仞娂ꚡ", false, "%E3%BA%83%EB%99%A6%E4%BB%9E%E5%A8%82%EA%9A%A1"},
		{"嶑쯫馎띫", false, "%E5%B6%91%EC%AF%AB%E9%A6%8E%EB%9D%AB"},
		{"絨뀿鴁뇨ះ", false, "%E7%B5%A8%EB%80%BF%E9%B4%81%EB%87%A8%E1%9F%87"},
		{"譽ᝃ幒睆", false, "%E8%AD%BD%E1%9D%83%E5%B9%92%E7%9D%86"},
		{"ꖉꍯ偾뭸⤓墴", false, "%EA%96%89%EA%8D%AF%E5%81%BE%EB%AD%B8%E2%A4%93%E5%A2%B4"},
		{"酛ꕒ쮖嬏㗛", false, "%E9%85%9B%EA%95%92%EC%AE%96%E5%AC%8F%E3%97%9B"},
		{"䜗粖ꞓ", false, "%E4%9C%97%E7%B2%96%EA%9E%93"},
		{"ᅣ䌓鳕", false, "%E1%85%A3%E4%8C%93%E9%B3%95"},
		{"✮䶕㝆仫", false, "%E2%9C%AE%E4%B6%95%E3%9D%86%E4%BB%AB"},
		{"", false, ""},
		{"鑘ቊꚌॉ뜶ᱣ끎ሩ", false, "%E9%91%98%E1%89%8A%EA%9A%8C%E0%A5%89%EB%9C%B6%E1%B1%A3%EB%81%8E%E1%88%A9"},
		{"窳ポퟬ䒉∧", false, "%E7%AA%B3%E3%83%9D%ED%9F%AC%E4%92%89%E2%88%A7"},
		{"룢䆌", false, "%EB%A3%A2%E4%86%8C"},
		{"", true, ""},
		{"羌鍝豁퍱梫ꮢ䟨労滊䘪⡩", true, "%E7%BE%8C%E9%8D%9D%E8%B1%81%ED%8D%B1%E6%A2%AB%EA%AE%A2%E4%9F%A8%E5%8A%B4%E6%BB%8A%E4%98%AA%E2%A1%A9"},
		{"틞", true, "%ED%8B%9E"},
		{"䯧鞈蓠哻뮉奼", true, "%E4%AF%A7%E9%9E%88%E8%93%A0%E5%93%BB%EB%AE%89%E5%A5%BC"},
		{"簦ꣁ숻玥ᔫ}夓栖㈤赧哼", true, "%E7%B0%A6%EA%A3%81%EC%88%BB%E7%8E%A5%E1%94%AB}%E5%A4%93%E6%A0%96%E3%88%A4%E8%B5%A7%E5%93%BC"},
		{"ꃍ纬᾽", true, "%EA%83%8D%E7%BA%AC%E1%BE%BD"},
		{"殿⺗䧽", true, "%E6%AE%BF%E2%BA%97%E4%A7%BD"},
		{"헋尨哩鳪ጭ컻꾓ꦜ覩", true, "%ED%97%8B%E5%B0%A8%E5%93%A9%E9%B3%AA%E1%8C%AD%EC%BB%BB%EA%BE%93%EA%A6%9C%E8%A6%A9"},
		{"欈䖳薙壢轋飀뷡䗼翱縀", true, "%E6%AC%88%E4%96%B3%E8%96%99%E5%A3%A2%E8%BD%8B%E9%A3%80%EB%B7%A1%E4%97%BC%E7%BF%B1%E7%B8%80"},
		{"㕙", true, "%E3%95%99"},
		{"츣膶㏚甧舸ㅘ뛓釫趔쨩深", true, "%EC%B8%A3%E8%86%B6%E3%8F%9A%E7%94%A7%E8%88%B8%E3%85%98%EB%9B%93%E9%87%AB%E8%B6%94%EC%A8%A9%E6%B7%B1"},
		{"䬹", true, "%E4%AC%B9"},
		{"ꀑꪸ놛곈渆⚂", true, "%EA%80%91%EA%AA%B8%EB%86%9B%EA%B3%88%E6%B8%86%E2%9A%82"},
		{"垹ㅳ瓑", true, "%E5%9E%B9%E3%85%B3%E7%93%91"},
		{"牰ꃊ腓", true, "%E7%89%B0%EA%83%8A%E8%85%93"},
		{"ⴖ택ݔ洔⦿ꚛ뎜鱝ᙡ㧢", true, "%E2%B4%96%ED%83%9D%DD%94%E6%B4%94%E2%A6%BF%EA%9A%9B%EB%8E%9C%E9%B1%9D%E1%99%A1%E3%A7%A2"},
		{"葓㞝代Ữ韍Ꭵ䏔", true, "%E8%91%93%E3%9E%9D%E4%BB%A3%E1%BB%AE%E9%9F%8D%E1%8E%A5%E4%8F%94"},
	}
	for i, c := range cases {
		set := NON_ALPHANUMERIC
		if c.controls {
			set = CONTROLS
		}
		if got := Encode(c.s, set); got != c.want {
			t.Errorf("case %d: %q != %q", i, got, c.want)
		}
	}
}
func TestAsciiSetValueSemantics(t *testing.T) {
	original := CONTROLS
	changed := original.Add(' ').Add('/').Remove('/')
	if got, want := Encode("a b/c", changed), "a%20b/c"; got != want {
		t.Fatalf("Encode with the changed set = %q, want %q", got, want)
	}
	if original != CONTROLS || Encode("a b/c", original) != "a b/c" {
		t.Fatal("Add/Remove mutated original")
	}
}
