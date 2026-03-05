package space

// Vec3 represents a 3D point or vector.
type Vec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

// AABB represents an axis-aligned bounding box.
type AABB struct {
	Min Vec3 `json:"min"`
	Max Vec3 `json:"max"`
}

// Width returns the box extent along X.
func (a AABB) Width() float64 { return a.Max.X - a.Min.X }

// Height returns the box extent along Y.
func (a AABB) Height() float64 { return a.Max.Y - a.Min.Y }

// Depth returns the box extent along Z.
func (a AABB) Depth() float64 { return a.Max.Z - a.Min.Z }

// Volume returns the volume of the box.
func (a AABB) Volume() float64 { return a.Width() * a.Height() * a.Depth() }

// Center returns the center point of the box.
func (a AABB) Center() Vec3 {
	return Vec3{
		X: (a.Min.X + a.Max.X) / 2,
		Y: (a.Min.Y + a.Max.Y) / 2,
		Z: (a.Min.Z + a.Max.Z) / 2,
	}
}

// Valid returns true if min < max on all axes.
func (a AABB) Valid() bool {
	return a.Min.X < a.Max.X && a.Min.Y < a.Max.Y && a.Min.Z < a.Max.Z
}

// Intersects returns true if two AABBs overlap (exclusive — touching edges don't count).
func (a AABB) Intersects(b AABB) bool {
	return a.Min.X < b.Max.X && a.Max.X > b.Min.X &&
		a.Min.Y < b.Max.Y && a.Max.Y > b.Min.Y &&
		a.Min.Z < b.Max.Z && a.Max.Z > b.Min.Z
}

// Contains returns true if a fully contains b.
func (a AABB) Contains(b AABB) bool {
	return a.Min.X <= b.Min.X && a.Max.X >= b.Max.X &&
		a.Min.Y <= b.Min.Y && a.Max.Y >= b.Max.Y &&
		a.Min.Z <= b.Min.Z && a.Max.Z >= b.Max.Z
}

// ContainsPoint returns true if the point is inside the box.
func (a AABB) ContainsPoint(p Vec3) bool {
	return p.X >= a.Min.X && p.X <= a.Max.X &&
		p.Y >= a.Min.Y && p.Y <= a.Max.Y &&
		p.Z >= a.Min.Z && p.Z <= a.Max.Z
}

// NewAABB creates an AABB from position and dimensions.
func NewAABB(pos Vec3, width, height, depth float64) AABB {
	return AABB{
		Min: pos,
		Max: Vec3{X: pos.X + width, Y: pos.Y + height, Z: pos.Z + depth},
	}
}
