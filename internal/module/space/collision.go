package space

import "fmt"

// CollisionError is returned when a placement would cause a collision.
type CollisionError struct {
	PlacedID    string
	CollidesID  string
	PlacedBox   AABB
	CollidesBox AABB
}

func (e *CollisionError) Error() string {
	return fmt.Sprintf("collision: object %s intersects with %s", e.PlacedID, e.CollidesID)
}

// OutOfBoundsError is returned when a placement is outside the parent space.
type OutOfBoundsError struct {
	ObjectID string
	Object   AABB
	Space    AABB
}

func (e *OutOfBoundsError) Error() string {
	return fmt.Sprintf("out of bounds: object %s does not fit within parent space", e.ObjectID)
}

// Placement represents an object with an ID and bounding box.
type Placement struct {
	ID   string
	Box  AABB
}

// ValidatePlacement checks that newObj fits within parentBounds and does not
// collide with any existing siblings. Returns nil if placement is valid.
func ValidatePlacement(newObj Placement, parentBounds *AABB, siblings []Placement) error {
	if !newObj.Box.Valid() {
		return fmt.Errorf("invalid bounds for object %s: min must be less than max on all axes", newObj.ID)
	}

	if parentBounds != nil {
		if !parentBounds.Contains(newObj.Box) {
			return &OutOfBoundsError{
				ObjectID: newObj.ID,
				Object:   newObj.Box,
				Space:    *parentBounds,
			}
		}
	}

	for _, sib := range siblings {
		if sib.ID == newObj.ID {
			continue
		}
		if newObj.Box.Intersects(sib.Box) {
			return &CollisionError{
				PlacedID:    newObj.ID,
				CollidesID:  sib.ID,
				PlacedBox:   newObj.Box,
				CollidesBox: sib.Box,
			}
		}
	}

	return nil
}

// FindCollisions returns all pairs of colliding objects in the list.
func FindCollisions(objects []Placement) [][2]Placement {
	var collisions [][2]Placement
	for i := 0; i < len(objects); i++ {
		for j := i + 1; j < len(objects); j++ {
			if objects[i].Box.Intersects(objects[j].Box) {
				collisions = append(collisions, [2]Placement{objects[i], objects[j]})
			}
		}
	}
	return collisions
}
