package tui

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestMenuDialogCentersTitleToPopupBox(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	view := plain(newMenuDialog().View())
	lines := strings.Split(view, "\n")
	titleLine := ""
	for _, line := range lines {
		if strings.Contains(line, "//// MENU ////") {
			titleLine = line
			break
		}
	}
	if titleLine == "" {
		t.Fatalf("expected menu title in popup, got %q", view)
	}

	titleCol := strings.Index(titleLine, "//// MENU ////")
	if titleCol < 8 {
		t.Fatalf("expected centered menu title, got column %d in %q", titleCol, titleLine)
	}

	rightPad := len(titleLine) - titleCol - len("//// MENU ////")
	if diff := titleCol - rightPad; diff < -2 || diff > 2 {
		t.Fatalf("expected title to be centered within popup box, left=%d right=%d line=%q", titleCol, rightPad, titleLine)
	}
}

func TestMenuDialogKeepsItemsLeftAligned(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	view := plain(newMenuDialog().View())
	lines := strings.Split(view, "\n")
	titleCol := -1
	themesCol := -1
	for _, line := range lines {
		if titleCol == -1 && strings.Contains(line, "//// MENU ////") {
			titleCol = strings.Index(line, "//// MENU ////")
		}
		if themesCol == -1 && strings.Contains(line, "Themes") {
			themesCol = strings.Index(line, "Themes")
		}
	}
	if titleCol == -1 || themesCol == -1 {
		t.Fatalf("expected both title and menu item in popup, got %q", view)
	}
	if themesCol >= titleCol {
		t.Fatalf("expected menu items to stay left aligned beneath centered title, title col=%d item col=%d view=%q", titleCol, themesCol, view)
	}
}
