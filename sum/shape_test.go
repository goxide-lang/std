package sum

import "testing"

func TestMalformedPrivateTags(t *testing.T) {
	if TagOption(Option[int]{tag: 2}) != 2 || TagResult(Result[int, string]{tag: 3}) != 3 || TagResult(Result[int, string]{}) != 0 {
		t.Fatal("raw tags were validated or changed")
	}
	for name, f := range map[string]func(){
		"option": func() { ValidateOption(Option[int]{tag: 2}) },
		"result": func() { ValidateResult(Result[int, string]{tag: 3}) },
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("invalid tag accepted")
				}
			}()
			f()
		})
	}
}
