package answer

import (
	"github.com/ggmolly/belfast/internal/answer/appreciate"
	"github.com/ggmolly/belfast/internal/connection"
)

func UnlockAppreciateMusic(buffer *[]byte, client *connection.Client) (int, int, error) {
	return appreciate.UnlockAppreciateMusic(buffer, client)
}

func ToggleAppreciationMusicLike(buffer *[]byte, client *connection.Client) (int, int, error) {
	return appreciate.ToggleAppreciationMusicLike(buffer, client)
}

func UpdateAppreciationMusicPlayerSettings(buffer *[]byte, client *connection.Client) (int, int, error) {
	return appreciate.UpdateAppreciationMusicPlayerSettings(buffer, client)
}

func UnlockAppreciateGallery(buffer *[]byte, client *connection.Client) (int, int, error) {
	return appreciate.UnlockAppreciateGallery(buffer, client)
}

func ToggleAppreciationGalleryLike(buffer *[]byte, client *connection.Client) (int, int, error) {
	return appreciate.ToggleAppreciationGalleryLike(buffer, client)
}
