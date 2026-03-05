package space

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAABB_Intersects(t *testing.T) {
	a := NewAABB(Vec3{0, 0, 0}, 2, 2, 2)

	tests := []struct {
		name   string
		b      AABB
		expect bool
	}{
		{"overlapping", NewAABB(Vec3{1, 1, 1}, 2, 2, 2), true},
		{"no overlap X", NewAABB(Vec3{3, 0, 0}, 2, 2, 2), false},
		{"no overlap Y", NewAABB(Vec3{0, 3, 0}, 2, 2, 2), false},
		{"no overlap Z", NewAABB(Vec3{0, 0, 3}, 2, 2, 2), false},
		{"touching edge (not intersecting)", NewAABB(Vec3{2, 0, 0}, 2, 2, 2), false},
		{"contained", NewAABB(Vec3{0.5, 0.5, 0.5}, 1, 1, 1), true},
		{"identical", NewAABB(Vec3{0, 0, 0}, 2, 2, 2), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, a.Intersects(tt.b))
		})
	}
}

func TestAABB_Contains(t *testing.T) {
	parent := NewAABB(Vec3{0, 0, 0}, 10, 10, 10)

	assert.True(t, parent.Contains(NewAABB(Vec3{1, 1, 1}, 2, 2, 2)))
	assert.True(t, parent.Contains(NewAABB(Vec3{0, 0, 0}, 10, 10, 10)))
	assert.False(t, parent.Contains(NewAABB(Vec3{-1, 0, 0}, 2, 2, 2)))
	assert.False(t, parent.Contains(NewAABB(Vec3{9, 9, 9}, 2, 2, 2)))
}

func TestAABB_Properties(t *testing.T) {
	a := NewAABB(Vec3{1, 2, 3}, 4, 5, 6)
	assert.Equal(t, 4.0, a.Width())
	assert.Equal(t, 5.0, a.Height())
	assert.Equal(t, 6.0, a.Depth())
	assert.Equal(t, 120.0, a.Volume())
	assert.Equal(t, Vec3{3, 4.5, 6}, a.Center())
	assert.True(t, a.Valid())
}

func TestAABB_Invalid(t *testing.T) {
	a := AABB{Min: Vec3{5, 5, 5}, Max: Vec3{3, 3, 3}}
	assert.False(t, a.Valid())
}

func TestValidatePlacement_FitsInParent(t *testing.T) {
	parent := NewAABB(Vec3{0, 0, 0}, 10, 10, 10)
	obj := Placement{ID: "table-1", Box: NewAABB(Vec3{1, 0, 1}, 2, 1, 2)}

	err := ValidatePlacement(obj, &parent, nil)
	require.NoError(t, err)
}

func TestValidatePlacement_OutOfBounds(t *testing.T) {
	parent := NewAABB(Vec3{0, 0, 0}, 10, 10, 10)
	obj := Placement{ID: "table-1", Box: NewAABB(Vec3{9, 0, 9}, 2, 1, 2)}

	err := ValidatePlacement(obj, &parent, nil)
	require.Error(t, err)

	var oob *OutOfBoundsError
	assert.ErrorAs(t, err, &oob)
	assert.Equal(t, "table-1", oob.ObjectID)
}

func TestValidatePlacement_Collision(t *testing.T) {
	parent := NewAABB(Vec3{0, 0, 0}, 10, 10, 10)
	existing := []Placement{
		{ID: "table-1", Box: NewAABB(Vec3{2, 0, 2}, 2, 1, 2)},
		{ID: "table-2", Box: NewAABB(Vec3{6, 0, 6}, 2, 1, 2)},
	}
	newObj := Placement{ID: "table-3", Box: NewAABB(Vec3{3, 0, 3}, 2, 1, 2)}

	err := ValidatePlacement(newObj, &parent, existing)
	require.Error(t, err)

	var ce *CollisionError
	assert.ErrorAs(t, err, &ce)
	assert.Equal(t, "table-3", ce.PlacedID)
	assert.Equal(t, "table-1", ce.CollidesID)
}

func TestValidatePlacement_NoCollision(t *testing.T) {
	parent := NewAABB(Vec3{0, 0, 0}, 10, 10, 10)
	existing := []Placement{
		{ID: "table-1", Box: NewAABB(Vec3{0, 0, 0}, 2, 1, 2)},
		{ID: "table-2", Box: NewAABB(Vec3{6, 0, 6}, 2, 1, 2)},
	}
	newObj := Placement{ID: "table-3", Box: NewAABB(Vec3{3, 0, 3}, 2, 1, 2)}

	err := ValidatePlacement(newObj, &parent, existing)
	require.NoError(t, err)
}

func TestValidatePlacement_SkipsSelf(t *testing.T) {
	obj := Placement{ID: "table-1", Box: NewAABB(Vec3{0, 0, 0}, 2, 1, 2)}
	siblings := []Placement{obj}

	err := ValidatePlacement(obj, nil, siblings)
	require.NoError(t, err)
}

func TestFindCollisions(t *testing.T) {
	objects := []Placement{
		{ID: "a", Box: NewAABB(Vec3{0, 0, 0}, 3, 3, 3)},
		{ID: "b", Box: NewAABB(Vec3{2, 2, 2}, 3, 3, 3)},
		{ID: "c", Box: NewAABB(Vec3{10, 10, 10}, 1, 1, 1)},
	}

	collisions := FindCollisions(objects)
	assert.Len(t, collisions, 1)
	assert.Equal(t, "a", collisions[0][0].ID)
	assert.Equal(t, "b", collisions[0][1].ID)
}

func TestValidatePlacement_InvalidBounds(t *testing.T) {
	obj := Placement{ID: "bad", Box: AABB{Min: Vec3{5, 5, 5}, Max: Vec3{3, 3, 3}}}
	err := ValidatePlacement(obj, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid bounds")
}
