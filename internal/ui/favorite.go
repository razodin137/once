package ui

const (
	favoriteStar   = "★"
	unfavoriteStar = "☆"
)

func favoriteStarText(favorite bool) string {
	if favorite {
		return favoriteStar
	}
	return unfavoriteStar
}
