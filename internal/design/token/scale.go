package token

type (
	Space   struct{ px float32 }
	Size    struct{ px float32 }
	Radius  struct{ px float32 }
	Stroke  struct{ px float32 }
	Opacity struct{ v float32 }
)

func (s Space) Px() float32 { return s.px }

func (s Size) Px() float32 { return s.px }

func (r Radius) Px() float32 { return r.px }

func (s Stroke) Px() float32 { return s.px }

func (o Opacity) Value() float32 { return o.v }

var (
	Space0  = Space{}
	Space2  = Space{2}
	Space4  = Space{4}
	Space6  = Space{6}
	Space8  = Space{8}
	Space12 = Space{12}
	Space16 = Space{16}
	Space20 = Space{20}
	Space24 = Space{24}
	Space32 = Space{32}
)

var (
	Radius0    = Radius{}
	Radius2    = Radius{2}
	Radius4    = Radius{4}
	Radius6    = Radius{6}
	RadiusFull = Radius{999}
)

var (
	Stroke1 = Stroke{1}
	Stroke2 = Stroke{2}
)
