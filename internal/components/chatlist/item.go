package chatlist

type Item interface {
	Render(width int) string
}
