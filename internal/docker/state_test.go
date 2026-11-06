package docker

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBackupDue(t *testing.T) {
	t.Run("due when app has no state", func(t *testing.T) {
		s := &State{}
		assert.True(t, s.BackupDue("myapp"))
	})

	t.Run("due when no last backup", func(t *testing.T) {
		s := &State{Apps: map[string]*AppState{"myapp": {}}}
		assert.True(t, s.BackupDue("myapp"))
	})

	t.Run("due when last backup exceeds interval", func(t *testing.T) {
		s := &State{Apps: map[string]*AppState{
			"myapp": {LastBackup: OperationResult{At: time.Now().Add(-AutomaticTaskInterval - time.Minute)}},
		}}
		assert.True(t, s.BackupDue("myapp"))
	})

	t.Run("not due when last backup was recent and successful", func(t *testing.T) {
		s := &State{Apps: map[string]*AppState{
			"myapp": {LastBackup: OperationResult{At: time.Now().Add(-time.Second)}},
		}}
		assert.False(t, s.BackupDue("myapp"))
	})

	t.Run("due when last backup was recent but had an error", func(t *testing.T) {
		s := &State{Apps: map[string]*AppState{
			"myapp": {LastBackup: OperationResult{At: time.Now().Add(-time.Second), Error: "disk full"}},
		}}
		assert.True(t, s.BackupDue("myapp"))
	})
}

func TestUpdateDue(t *testing.T) {
	t.Run("due when app has no state", func(t *testing.T) {
		s := &State{}
		assert.True(t, s.UpdateDue("myapp"))
	})

	t.Run("not due when last update was recent and successful", func(t *testing.T) {
		s := &State{Apps: map[string]*AppState{
			"myapp": {LastUpdate: OperationResult{At: time.Now().Add(-time.Second)}},
		}}
		assert.False(t, s.UpdateDue("myapp"))
	})

	t.Run("due when last update was recent but had an error", func(t *testing.T) {
		s := &State{Apps: map[string]*AppState{
			"myapp": {LastUpdate: OperationResult{At: time.Now().Add(-time.Second), Error: "pull failed"}},
		}}
		assert.True(t, s.UpdateDue("myapp"))
	})
}

func TestRecordBackup(t *testing.T) {
	t.Run("records successful backup", func(t *testing.T) {
		s := &State{}
		s.RecordBackup("myapp", nil)

		assert.False(t, s.Apps["myapp"].LastBackup.At.IsZero())
		assert.Empty(t, s.Apps["myapp"].LastBackup.Error)
		assert.WithinDuration(t, time.Now(), s.Apps["myapp"].LastBackup.At, time.Second)
	})

	t.Run("records failed backup", func(t *testing.T) {
		s := &State{}
		s.RecordBackup("myapp", errors.New("disk full"))

		assert.Equal(t, "disk full", s.Apps["myapp"].LastBackup.Error)
		assert.WithinDuration(t, time.Now(), s.Apps["myapp"].LastBackup.At, time.Second)
	})
}

func TestRecordUpdate(t *testing.T) {
	t.Run("records successful update", func(t *testing.T) {
		s := &State{}
		s.RecordUpdate("myapp", nil)

		assert.False(t, s.Apps["myapp"].LastUpdate.At.IsZero())
		assert.Empty(t, s.Apps["myapp"].LastUpdate.Error)
	})

	t.Run("records failed update", func(t *testing.T) {
		s := &State{}
		s.RecordUpdate("myapp", errors.New("pull failed"))

		assert.Equal(t, "pull failed", s.Apps["myapp"].LastUpdate.Error)
	})
}

func TestRecordScreenshot(t *testing.T) {
	t.Run("records successful screenshot", func(t *testing.T) {
		s := &State{}
		s.RecordScreenshot("myapp", nil)

		assert.False(t, s.Apps["myapp"].LastScreenshot.At.IsZero())
		assert.Empty(t, s.Apps["myapp"].LastScreenshot.Error)
	})

	t.Run("records failed screenshot", func(t *testing.T) {
		s := &State{}
		s.RecordScreenshot("myapp", errors.New("browser timed out"))

		assert.Equal(t, "browser timed out", s.Apps["myapp"].LastScreenshot.Error)
	})
}

func TestFavorites(t *testing.T) {
	t.Run("not favorite without state", func(t *testing.T) {
		s := &State{}
		assert.False(t, s.IsFavorite("myapp"))
	})

	t.Run("set and clear favorite", func(t *testing.T) {
		s := &State{}

		s.SetFavorite("myapp", true)
		assert.True(t, s.IsFavorite("myapp"))
		assert.True(t, s.Apps["myapp"].Favorite)

		s.SetFavorite("myapp", false)
		assert.False(t, s.IsFavorite("myapp"))
	})

	t.Run("favorites survive a marshal round trip", func(t *testing.T) {
		s := &State{}
		s.SetFavorite("myapp", true)

		data, err := json.Marshal(s)
		assert.NoError(t, err)

		var restored State
		assert.NoError(t, json.Unmarshal(data, &restored))
		assert.True(t, restored.IsFavorite("myapp"))
	})
}
