package chatlist

import (
	"strings"
)

type List struct {
	width, height int
	items         []Item
	gap           int
	offsetIdx     int
	offsetLine    int
	follow        bool
}

type renderedItem struct {
	content string
	height  int
}

func NewList(items ...Item) *List {
	return &List{items: items, gap: 1}
}

func (l *List) SetSize(width, height int) {
	l.width = width
	l.height = height
}

func (l *List) SetGap(gap int) {
	l.gap = gap
}

func (l *List) Len() int {
	return len(l.items)
}

func (l *List) OffsetIdx() int {
	return l.offsetIdx
}

func (l *List) OffsetLine() int {
	return l.offsetLine
}

func (l *List) SetHeight(h int) {
	l.height = h
}

func (l *List) Height() int {
	return l.height
}

func (l *List) TotalOffset() int {
	var total int
	for i := 0; i < l.offsetIdx; i++ {
		total += l.itemHeight(l.items[i])
		if l.gap > 0 {
			total += l.gap
		}
	}
	return total - l.offsetLine
}

func (l *List) AtBottom() bool {
	if len(l.items) == 0 {
		return true
	}
	if l.offsetIdx >= len(l.items)-1 {
		return l.lastVisibleContent() <= l.height
	}
	remaining := l.remainingContent()
	visible := remaining - l.offsetLine
	return visible <= l.height
}

func (l *List) lastVisibleContent() int {
	var total int
	for i := l.offsetIdx; i < len(l.items); i++ {
		total += l.itemHeight(l.items[i])
		if l.gap > 0 && i > l.offsetIdx {
			total += l.gap
		}
	}
	return total - l.offsetLine
}

func (l *List) remainingContent() int {
	if len(l.items) == 0 {
		return 0
	}
	var total int
	for i := l.offsetIdx; i < len(l.items); i++ {
		total += l.itemHeight(l.items[i])
		if l.gap > 0 && i > l.offsetIdx {
			total += l.gap
		}
	}
	return total
}

func (l *List) ScrollBy(lines int) {
	if len(l.items) == 0 || lines == 0 {
		return
	}

	if lines > 0 {
		if l.AtBottom() {
			return
		}
		l.offsetLine += lines
		currentItem := l.getItem(l.offsetIdx)
		for l.offsetLine >= currentItem.height {
			l.offsetLine -= currentItem.height
			if l.gap > 0 {
				l.offsetLine = max(0, l.offsetLine-l.gap)
			}
			l.offsetIdx++
			if l.offsetIdx > len(l.items)-1 {
				l.ScrollToBottom()
				return
			}
			currentItem = l.getItem(l.offsetIdx)
		}

		lastOffsetIdx, lastOffsetLine, _ := l.lastOffsetItem()
		if l.offsetIdx > lastOffsetIdx || (l.offsetIdx == lastOffsetIdx && l.offsetLine > lastOffsetLine) {
			l.offsetIdx = lastOffsetIdx
			l.offsetLine = lastOffsetLine
		}
	} else {
		l.offsetLine += lines
		for l.offsetLine < 0 {
			l.offsetIdx--
			if l.offsetIdx < 0 {
				l.ScrollToTop()
				break
			}
			prevItem := l.getItem(l.offsetIdx)
			totalHeight := prevItem.height
			if l.gap > 0 {
				totalHeight += l.gap
			}
			l.offsetLine += totalHeight
		}
	}
}

func (l *List) ScrollToTop() {
	l.offsetIdx = 0
	l.offsetLine = 0
}

func (l *List) ScrollToBottom() {
	if len(l.items) == 0 {
		return
	}
	lastOffsetIdx, lastOffsetLine, _ := l.lastOffsetItem()
	l.offsetIdx = lastOffsetIdx
	l.offsetLine = lastOffsetLine
}

func (l *List) AppendItem(item Item) {
	l.items = append(l.items, item)
	if l.follow {
		l.ScrollToBottom()
	}
}

func (l *List) UpdateItem(idx int, item Item) {
	if idx < 0 || idx >= len(l.items) {
		return
	}
	l.items[idx] = item
}

func (l *List) TrimLast(n int) {
	if n <= 0 {
		return
	}
	if n > len(l.items) {
		n = len(l.items)
	}
	l.items = l.items[:len(l.items)-n]
}

func (l *List) SetFollow(follow bool) {
	l.follow = follow
}

func (l *List) Render() string {
	if len(l.items) == 0 {
		return ""
	}

	var lines []string
	currentIdx := l.offsetIdx
	currentOffset := l.offsetLine

	linesNeeded := l.height

	for linesNeeded > 0 && currentIdx < len(l.items) {
		item := l.getItem(currentIdx)
		itemLines := strings.Split(item.content, "\n")
		itemHeight := len(itemLines)

		if currentOffset >= 0 && currentOffset < itemHeight {
			lines = append(lines, itemLines[currentOffset:]...)
			if l.gap > 0 {
				for i := 0; i < l.gap; i++ {
					lines = append(lines, "")
				}
			}
		}

		linesNeeded = l.height - len(lines)
		currentIdx++
		currentOffset = 0
	}

	if len(lines) > l.height {
		lines = lines[:l.height]
	}

	return strings.Join(lines, "\n")
}

func (l *List) lastOffsetItem() (int, int, int) {
	var totalHeight int
	var idx int
	for idx = len(l.items) - 1; idx >= 0; idx-- {
		item := l.items[idx]
		itemHeight := l.itemHeight(item)
		if l.gap > 0 && idx < len(l.items)-1 {
			itemHeight += l.gap
		}
		totalHeight += itemHeight
		if totalHeight > l.height {
			break
		}
	}

	lineOffset := max(totalHeight-l.height, 0)
	idx = max(idx, 0)

	return idx, lineOffset, totalHeight
}

func (l *List) getItem(idx int) renderedItem {
	if idx < 0 || idx >= len(l.items) {
		return renderedItem{}
	}
	item := l.items[idx]
	rendered := item.Render(l.width)
	rendered = strings.TrimRight(rendered, "\n")
	height := strings.Count(rendered, "\n") + 1
	return renderedItem{content: rendered, height: height}
}

func (l *List) itemHeight(item Item) int {
	rendered := item.Render(l.width)
	rendered = strings.TrimRight(rendered, "\n")
	return strings.Count(rendered, "\n") + 1
}
