package space

import "testing"

// pathFromBytes derives a 2D integer path deterministically from bytes.
func pathFromBytes(data []byte) Path {
	if len(data) < 4 {
		return Path{}
	}
	return NewPath(
		[]float64{float64(int(data[0]%7) - 3), float64(int(data[1]%7) - 3)},
		[]float64{float64(int(data[2]%7) - 3), float64(int(data[3]%7) - 3)},
	)
}

func FuzzSpan(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Fuzz(func(t *testing.T, data []byte) {
		p := pathFromBytes(data)
		if p.Dims() == 0 {
			return
		}
		for _, b := range boxesFromBytes(data) {
			span, ok := p.Span(b)
			if ok != !span.Empty() {
				t.Fatalf("ok must mean positive length: span=%+v ok=%t box=%v", span, ok, b)
			}
			if !ok {
				continue
			}
			if span.Start < 0 || span.End > 1 || span.Start >= span.End {
				t.Fatalf("span out of range: %+v box=%v path=%v->%v", span, b, p.From, p.To)
			}
		}
	})
}

func FuzzTraverse(f *testing.F) {
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7})
	f.Fuzz(func(t *testing.T, data []byte) {
		p := pathFromBytes(data)
		if p.Dims() == 0 {
			return
		}
		boxes := boxesFromBytes(data)
		segs := Traverse(boxes, p)
		if len(segs) == 0 {
			t.Fatalf("traversal must never be empty")
		}
		if segs[0].Start != 0 || segs[len(segs)-1].End != 1 {
			t.Fatalf("traversal must tile [0,1]: first=%+v last=%+v", segs[0], segs[len(segs)-1])
		}
		for i, s := range segs {
			if s.Start >= s.End {
				t.Fatalf("segment %d has no positive length: %+v", i, s)
			}
			if s.Covered != (len(s.Boxes) > 0) {
				t.Fatalf("segment %d covered/boxes disagree: %+v", i, s)
			}
			if i > 0 && segs[i-1].End != s.Start {
				t.Fatalf("segment %d does not abut its predecessor: %+v then %+v", i, segs[i-1], s)
			}
		}
	})
}
