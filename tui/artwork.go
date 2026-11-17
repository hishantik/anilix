package tui

import (
	tea "charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/tui/ui"
)

// The cover art.
//
// A terminal shows a picture in one of two ways: the kitty graphics
// protocol transmits the image to the terminal, which draws it in a cell,
// or the image is rendered as coloured half blocks, which are plain text.
// Both are text, so the cover travels through the same buffer as the rest
// of the screen and needs no special case in the renderer.
//
// The cover is always rendered for the size it is going to be shown at. A
// cover that was rendered for another width would overflow the panel it
// lands in, so the rendered size is kept next to the image and a resize
// re-renders the cover instead of showing a stale one.

// coverTarget returns the size of the cover art of the current screen.
//
// The width is the outer width of the frame and the rows the outer height,
// the border included. A width of zero means the screen shows no artwork at
// all - a compact search screen has no panel to put it in.
func (m *SearchModel) coverTarget() (width, rows int) {
	switch m.state {
	case searchState:
		_, previewW, height, isNarrow := searchLayout(m.contentWidth, m.contentHeight)
		if isNarrow || previewW < 20 {
			return 0, 0
		}
		return previewW, m.previewArtworkRows(previewW, height)

	case animeState:
		return m.animeArtworkBox()

	case detailState:
		leftW, _, height, isNarrow := detailLayout(m.contentWidth, m.contentHeight)
		if isNarrow {
			return 0, 0
		}
		return leftW, m.detailArtworkRows(leftW, height)

	case watchlistState:
		_, previewW, height, isNarrow := watchlistLayout(m.contentWidth, m.contentHeight)
		if isNarrow || previewW < 20 {
			return 0, 0
		}
		return previewW, watchlistArtworkRows(m.watchlistSelected(), previewW, height)
	}

	return 0, 0
}

// refreshCoverForProtocolChange redraws the artwork on screen when the
// renderer learned a better protocol after the cover was rendered.
//
// The terminal answers the capability query while the first cover is
// already on its way, and a cover that was encoded by a weaker protocol
// should not wait for a screen change to be drawn by the better one.
func (m *SearchModel) refreshCoverForProtocolChange() tea.Cmd {
	if m.state != searchState && m.state != animeState && m.state != detailState {
		return nil
	}
	meta := m.searchState.Metadata
	if meta == nil || meta.CoverImage == "" {
		return nil
	}

	// Forget the size the cover was rendered at: ensureCover only renders
	// a cover that is missing or mis-sized for the panel it lands in.
	meta.CoverImage = ""
	meta.CoverWidth = 0
	meta.CoverRows = 0
	return m.coverCmd()
}

// clearCoverCmd drops the artwork the terminal still holds for the
// previous screen.
//
// The kitty protocol keeps a transmitted image in the memory of the
// terminal; a screen that no longer shows it should not pay for it. The
// other protocols are text and vanish with the frame they lived in.
func (m *SearchModel) clearCoverCmd() tea.Cmd {
	id := m.kittyImageID
	m.kittyImageID = 0
	return DeleteKittyImageCmd(id)
}

// ensureCover renders the cover of an entry when it is missing or when it
// was rendered for another size.
//
// The command is issued for the entry that is on screen: the cover of an
// entry that is not shown is never downloaded.
func (m *SearchModel) ensureCover(meta *MetadataPanel, index int) tea.Cmd {
	if meta == nil || meta.Cover == "" {
		return nil
	}

	width, rows := m.coverTarget()
	if width < 4 || rows < 3 {
		return nil
	}

	inner := width - 2
	innerRows := rows - 2
	if meta.CoverImage != "" && meta.CoverWidth == inner && meta.CoverRows == innerRows {
		return nil
	}

	return downloadCoverCmd(meta.Cover, inner, innerRows, index)
}

// renderArtwork renders the cover of an entry in the given size.
//
// The frame is drawn even when there is no image: a placeholder that keeps
// the shape of a poster is better than a hole in the layout.
func (m *SearchModel) renderArtwork(meta *MetadataPanel, width, rows int) string {
	if meta == nil || width < 4 || rows < 3 {
		return ""
	}

	title := meta.Title
	if title == "" {
		title = meta.TitleEnglish
	}

	return ui.Artwork{
		Image:  meta.CoverImage,
		Title:  title,
		Width:  width,
		Height: rows,
	}.Render()
}
