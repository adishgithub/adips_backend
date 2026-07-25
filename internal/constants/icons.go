package constants

// Icon/color IDs are plain ints stored in Postgres — Flutter owns the
// actual icon/color mapping (icons.dart / colors.dart), the backend
// only needs to know the valid *range* so it can reject garbage
// values on create/update instead of silently accepting any integer.
//
// If the Flutter-side icon or color palette grows, bump these two
// constants — nothing else needs to change.
const (
	MinIconID = 1
	MaxIconID = 80

	MinColorID = 1
	MaxColorID = 8
)

// ValidIconID reports whether id falls inside the allowed icon range.
func ValidIconID(id int) bool {
	return id >= MinIconID && id <= MaxIconID
}

// ValidColorID reports whether id falls inside the allowed color range.
func ValidColorID(id int) bool {
	return id >= MinColorID && id <= MaxColorID
}
