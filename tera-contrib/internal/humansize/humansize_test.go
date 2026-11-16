// Reference vectors recorded from the original crate at its locked version (a sample of the full differential set).
package humansize

import (
	"testing"
)

func TestRustVectors(t *testing.T) {
	cases := []struct {
		n      uint64
		binary bool
		want   string
	}{
		{0, false, "0 B"},
		{1, false, "1 B"},
		{2, false, "2 B"},
		{3, false, "3 B"},
		{4, false, "4 B"},
		{5, false, "5 B"},
		{6, false, "6 B"},
		{7, false, "7 B"},
		{8, false, "8 B"},
		{9, false, "9 B"},
		{10, false, "10 B"},
		{11, false, "11 B"},
		{12, false, "12 B"},
		{13, false, "13 B"},
		{14, false, "14 B"},
		{15, false, "15 B"},
		{16, false, "16 B"},
		{17, false, "17 B"},
		{18, false, "18 B"},
		{19, false, "19 B"},
		{20, false, "20 B"},
		{21, false, "21 B"},
		{980, false, "980 B"},
		{981, false, "981 B"},
		{982, false, "982 B"},
		{985, false, "985 B"},
		{988, false, "988 B"},
		{991, false, "991 B"},
		{994, false, "994 B"},
		{997, false, "997 B"},
		{1018, false, "1.02 kB"},
		{1000007, false, "1.00 MB"},
		{999999996, false, "1000.00 MB"},
		{999999999985, false, "1000.00 GB"},
		{1000000000015, false, "1.00 TB"},
		{1000000000000004, false, "1.00 PB"},
		{999999999999999993, false, "1 EB"},
		{2429894478011705556, false, "2.43 EB"},
		{17344294200118164946, false, "17.34 EB"},
		{8083395298327768190, false, "8.08 EB"},
		{2150487083605375316, false, "2.15 EB"},
		{13645468801853577939, false, "13.65 EB"},
		{1767661490929297159, false, "1.77 EB"},
		{11427875210129132286, false, "11.43 EB"},
		{8132069438512291469, false, "8.13 EB"},
		{6923938961694818085, false, "6.92 EB"},
		{13909782890688418327, false, "13.91 EB"},
		{2226131845072109385, false, "2.23 EB"},
		{7606257698152384095, false, "7.61 EB"},
		{10254278673933456681, false, "10.25 EB"},
		{1049881668475957378, false, "1.05 EB"},
		{1004, true, "1004 B"},
		{1034, true, "1.01 KiB"},
		{1048575, true, "1024.00 KiB"},
		{1073741812, true, "1024.00 MiB"},
		{1073741842, true, "1.00 GiB"},
		{1099511627783, true, "1.00 TiB"},
		{1125899906842620, true, "1024.00 TiB"},
		{1152921504606846961, true, "1 EiB"},
		{1152921504606846991, true, "1 EiB"},
		{4746661979129683042, true, "4.12 EiB"},
		{9627211582020385791, true, "8.35 EiB"},
		{12497333140741213935, true, "10.84 EiB"},
		{4448260140269273949, true, "3.86 EiB"},
		{8396151649199332249, true, "7.28 EiB"},
		{18353730566128433224, true, "15.92 EiB"},
		{15916040850883357386, true, "13.80 EiB"},
		{16663172990933035715, true, "14.45 EiB"},
		{9431445719069681772, true, "8.18 EiB"},
		{8971264694973135733, true, "7.78 EiB"},
		{17590844163685334057, true, "15.26 EiB"},
		{13195688361011313527, true, "11.45 EiB"},
		{14145902842921764906, true, "12.27 EiB"},
	}
	for i, c := range cases {
		if got := FormatSize(c.n, c.binary); got != c.want {
			t.Errorf("case %d: %q != %q", i, got, c.want)
		}
	}
}
