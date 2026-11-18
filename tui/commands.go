package tui

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hishantik/anilix/aniskip"
	"github.com/hishantik/anilix/auth"
	"github.com/hishantik/anilix/player"
	"github.com/hishantik/anilix/provider/anilist"
	"github.com/hishantik/anilix/provider/jikan"
	"github.com/hishantik/anilix/source"
	"github.com/hishantik/anilix/update"

	tea "charm.land/bubbletea/v2"

	"github.com/hishantik/anilix/provider/hianime"
)

func (m *SearchModel) doSearch(query string) tea.Cmd {
	hianimeProvider := m.hianimeProvider

	return func() tea.Msg {
		results, err := hianimeSearchResults(hianimeProvider, query)
		if err != nil {
			return SearchErrorMsg{Err: err}
		}
		return SearchResultsMsg{Results: results}
	}
}

// resolveIDs fills AniList and MAL IDs for search results in the background,
// so the result list appears immediately instead of waiting for AniList.
func (m *SearchModel) resolveIDs(results []*source.Anime) tea.Cmd {
	anilistClient := m.anilistClient

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		resolveAnimeIDs(ctx, anilistClient, results)
		return IDsResolvedMsg{Results: results}
	}
}

// resolveIDsViaKitsu resolves AniList and MAL IDs for search results using
// Kitsu, whose single request carries both ids through the sideloaded
// mappings. It runs in the background and never blocks the result list or
// the metadata batch; the UI applies the ids only where AniList resolved
// none, so this never races with the AniList resolution.
func (m *SearchModel) resolveIDsViaKitsu(results []*source.Anime) tea.Cmd {
	kitsuClient := m.kitsuClient

	return func() tea.Msg {
		if kitsuClient == nil || len(results) == 0 {
			return MALIDsResolvedMsg{IDs: nil}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		titles := make([]string, 0, len(results))
		for _, a := range results {
			titles = append(titles, a.Name)
		}

		matches, err := kitsuClient.ResolveByTitles(ctx, titles)
		if err != nil {
			// Non-fatal: the ids are optional enrichment; silently skip.
			return MALIDsResolvedMsg{IDs: nil}
		}

		ids := make(map[int]ExternalIDs)
		for i, match := range matches {
			if match.Anime == nil || i >= len(results) {
				continue
			}
			ids[i] = ExternalIDs{
				AniListID: match.Anime.AniListID,
				MALID:     match.Anime.MALID,
			}
		}
		return MALIDsResolvedMsg{IDs: ids}
	}
}

// resolveAnimeIDs resolves AniList IDs (and MAL IDs) for the given results.
//
// AniList is the primary metadata provider: hianime gives no AniList ID, so
// titles are matched against AniList, which also returns the MAL ID used for
// AniSkip. Matching is exact-after-normalization, never fuzzy, because a
// near-match would attach the wrong anime.
func resolveAnimeIDs(ctx context.Context, anilistClient *anilist.Client, results []*source.Anime) {
	if len(results) == 0 || anilistClient == nil {
		return
	}

	titles := make([]string, 0, len(results))
	for _, a := range results {
		titles = append(titles, a.Name)
	}

	matches, err := anilistClient.ResolveByTitles(ctx, titles)
	if err != nil {
		log.Printf("[anilix] anilist id resolution failed: %v\n", err)
		return
	}
	for i, m := range matches {
		if m.Media == nil || i >= len(results) {
			continue
		}
		results[i].AniListID = m.Media.ID
		if m.Media.IDMal > 0 {
			results[i].MALID = m.Media.IDMal
		}
	}
}

// anyMissingMAL reports whether some result still lacks a MAL ID.
func anyMissingMAL(animes []*source.Anime) bool {
	for _, a := range animes {
		if a.MALID == 0 {
			return true
		}
	}
	return false
}

// hianimeSearchResults searches hianime. IDs (AniList/MAL) are resolved
// separately in the background so results show up immediately.
func hianimeSearchResults(hianimeProvider *hianime.Provider, query string) ([]*source.Anime, error) {
	found, err := hianimeProvider.Search(query)
	if err != nil || len(found) == 0 {
		if err != nil {
			log.Printf("[anilix] hianime search failed: %v\n", err)
		}
		return nil, err
	}

	queryLower := strings.ToLower(query)
	sort.SliceStable(found, func(i, j int) bool {
		nameI := strings.ToLower(found[i].Name)
		nameJ := strings.ToLower(found[j].Name)
		scoreI := matchScore(nameI, queryLower)
		scoreJ := matchScore(nameJ, queryLower)
		return scoreI > scoreJ
	})

	return found, nil
}

// fetchMetadataBatch fetches metadata for all results in a single AniList GraphQL call.
func (m *SearchModel) fetchMetadataBatch(results []*source.Anime) tea.Cmd {
	// Collect all AniList IDs with their result index
	type indexID struct {
		Index     int
		AniListID int
	}
	var pairs []indexID
	for i, r := range results {
		if r.AniListID > 0 {
			pairs = append(pairs, indexID{Index: i, AniListID: r.AniListID})
		}
	}

	if len(pairs) == 0 {
		return func() tea.Msg {
			return MetadataBatchLoadedMsg{MetadataMap: make(map[int]*MetadataPanel)}
		}
	}

	ids := make([]int, len(pairs))
	for i, p := range pairs {
		ids[i] = p.AniListID
	}

	anilistClient := m.anilistClient
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		batch, err := anilistClient.GetAnimeBatch(ctx, ids)
		if err != nil {
			return MetadataBatchLoadedMsg{MetadataMap: make(map[int]*MetadataPanel)}
		}

		resultMap := make(map[int]*MetadataPanel, len(pairs))
		for _, p := range pairs {
			if media, ok := batch[p.AniListID]; ok {
				resultMap[p.Index] = mergeMetadata(nil, media)
			}
		}
		return MetadataBatchLoadedMsg{MetadataMap: resultMap}
	}
}

// downloadCoverCmd downloads and renders a cover image for a given result index.
//
// The width and the rows are the size of the image itself, without the
// frame it is shown in: the cover is rendered for the panel it lands in, so
// a resize renders it again.
//
// The renderer is adaptive: the cover is rendered with the best protocol
// of the chain, and a protocol that fails is skipped for the session. A
// terminal without a picture protocol never downloads anything: the
// framed placeholder needs no image.
func downloadCoverCmd(coverURL string, width, rows, index int) tea.Cmd {
	if coverURL == "" {
		return nil
	}
	if !chainCanRenderPictures() {
		return nil
	}
	return func() tea.Msg {
		img, cachePath, err := DownloadCoverImage(coverURL)
		if err != nil {
			return CoverImageLoadedMsg{Rendered: "", Index: index}
		}
		result := RenderAdaptive(img, width, rows, cachePath, coverURL)
		return CoverImageLoadedMsg{
			Rendered:         result.Placeholder,
			KittyTransmitSeq: result.TransmitSeq,
			KittyImageID:     result.KittyImageID,
			Index:            index,
			Width:            width,
			Rows:             rows,
		}
	}
}

// chainCanRenderPictures reports whether any protocol of the chain draws
// an actual picture. The placeholder at the end of the chain does not:
// it is the reason the download would be wasted.
func chainCanRenderPictures() bool {
	for _, p := range rendererChain() {
		if p != ProtocolNone {
			return true
		}
	}
	return false
}

func (m *SearchModel) fetchMetadata() tea.Cmd {
	if len(m.searchState.Results) == 0 || m.searchState.Selected >= len(m.searchState.Results) {
		return func() tea.Msg {
			return MetadataLoadedMsg{Metadata: nil, Index: -1}
		}
	}

	anime := m.searchState.Results[m.searchState.Selected]
	idx := m.searchState.Selected

	// Prefer AniList (faster GraphQL API) — only fall back to Jikan if no AniList ID
	if anime.AniListID > 0 {
		anilistID := anime.AniListID
		anilistClient := m.anilistClient
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			data, err := anilistClient.GetAnime(ctx, anilistID)
			if err != nil {
				return MetadataLoadedMsg{Metadata: nil, Index: idx}
			}
			return MetadataLoadedMsg{Metadata: mergeMetadata(nil, data), Index: idx}
		}
	}

	if anime.MALID > 0 {
		malID := anime.MALID
		jikanClient := m.jikanClient
		return func() tea.Msg {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			data, err := jikanClient.GetAnime(ctx, malID)
			if err != nil {
				return MetadataLoadedMsg{Metadata: nil, Index: idx}
			}
			return MetadataLoadedMsg{Metadata: mergeMetadata(data, nil), Index: idx}
		}
	}

	return func() tea.Msg {
		return MetadataLoadedMsg{Metadata: nil, Index: idx}
	}
}

func mergeMetadata(jikanData *jikan.AnimeData, anilistData *anilist.MediaData) *MetadataPanel {
	panel := &MetadataPanel{}

	var sources []string
	if jikanData != nil {
		sources = append(sources, "Jikan")
	}
	if anilistData != nil {
		sources = append(sources, "AniList")
	}
	if len(sources) > 1 {
		panel.Source = "Jikan + AniList"
	} else if len(sources) == 1 {
		panel.Source = sources[0]
	}

	if anilistData != nil {
		panel.Title = anilistData.Title.Romaji
		panel.TitleEnglish = anilistData.Title.English
		panel.TitleNative = anilistData.Title.Native
		panel.Cover = anilistData.CoverImage.Large
		if panel.Cover == "" {
			panel.Cover = anilistData.CoverImage.Medium
		}
		if anilistData.Format != "" {
			panel.Type = anilistData.Format
		} else {
			panel.Type = anilistData.Type
		}
		panel.Status = anilistData.Status
		panel.Episodes = anilistData.Episodes
		panel.Score = float64(anilistData.AverageScore) / 10
		panel.Popularity = anilistData.Popularity
		panel.Genres = anilistData.Genres
		panel.Synopsis = anilistData.Description
		if anilistData.StartDate.Year != 0 {
			panel.Year = anilistData.StartDate.Year
		} else if anilistData.SeasonYear != 0 {
			panel.Year = anilistData.SeasonYear
		}
	}

	if jikanData != nil {
		if panel.Title == "" {
			if jikanData.TitleEnglish != "" {
				panel.Title = jikanData.TitleEnglish
			} else {
				panel.Title = jikanData.Title
			}
		}
		if panel.TitleEnglish == "" && jikanData.TitleEnglish != "" {
			panel.TitleEnglish = jikanData.TitleEnglish
		}
		if panel.Cover == "" {
			panel.Cover = jikanData.Images.JPG.LargeImageURL
			if panel.Cover == "" {
				panel.Cover = jikanData.Images.JPG.ImageURL
			}
		}
		if panel.Type == "" {
			panel.Type = jikanData.Type
		}
		if panel.Status == "" {
			panel.Status = jikanData.Status
		}
		if panel.Episodes == 0 {
			panel.Episodes = jikanData.Episodes
		}
		if panel.Score == 0 {
			panel.Score = jikanData.Score
		}
		if panel.Rank == 0 {
			panel.Rank = jikanData.Rank
		}
		if panel.Popularity == 0 {
			panel.Popularity = jikanData.Popularity
		}
		if panel.Year == 0 {
			if y, ok := jikanData.Year.(float64); ok {
				panel.Year = int(y)
			}
		}
		if len(panel.Genres) == 0 {
			genres := make([]string, 0, len(jikanData.Genres))
			for _, g := range jikanData.Genres {
				genres = append(genres, g.Name)
			}
			panel.Genres = genres
		}
		if panel.Synopsis == "" {
			panel.Synopsis = jikanData.Synopsis
		}
	}

	return panel
}

func (m *SearchModel) prefetchMetadata(ctx context.Context, start, end int) {
	results := m.searchState.Results
	if end > len(results) {
		end = len(results)
	}
	anilistClient := m.anilistClient
	jikanClient := m.jikanClient

	anilistIDs := make([]int, 0)
	for i := start; i < end; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		anime := results[i]
		if anime.AniListID > 0 && !anilistClient.IsCached(anime.AniListID) {
			anilistIDs = append(anilistIDs, anime.AniListID)
		}
	}

	if len(anilistIDs) > 0 {
		batchSize := 10
		for i := 0; i < len(anilistIDs); i += batchSize {
			select {
			case <-ctx.Done():
				return
			default:
			}
			batchEnd := i + batchSize
			if batchEnd > len(anilistIDs) {
				batchEnd = len(anilistIDs)
			}
			batch := anilistIDs[i:batchEnd]
			batchCtx, batchCancel := context.WithTimeout(ctx, 10*time.Second)
			if _, err := anilistClient.GetAnimeBatch(batchCtx, batch); err != nil && ctx.Err() == nil {
				log.Printf("[anilix] prefetch batch failed: %v\n", err)
			}
			batchCancel()
		}
	}

	malIDs := make([]int, 0)
	for i := start; i < end; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		anime := results[i]
		if anime.MALID > 0 && !jikanClient.IsCached(anime.MALID) {
			malIDs = append(malIDs, anime.MALID)
		}
	}

	for _, malID := range malIDs {
		select {
		case <-ctx.Done():
			return
		default:
		}
		dataCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		if _, err := jikanClient.GetAnime(dataCtx, malID); err != nil && ctx.Err() == nil {
			log.Printf("[anilix] prefetch jikan failed: %v\n", err)
		}
		cancel()
		time.Sleep(350 * time.Millisecond)
	}

	newTitles := make(map[int][]string)
	for i := start; i < end; i++ {
		select {
		case <-ctx.Done():
			return
		default:
		}
		anime := results[i]
		if anime.MALID == 0 {
			continue
		}
		dataCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		episodes, err := jikanClient.GetEpisodes(dataCtx, anime.MALID)
		cancel()
		if err == nil && len(episodes) > 0 {
			titles := make([]string, len(episodes))
			for j, ep := range episodes {
				titles[j] = ep.Title
			}
			newTitles[i] = titles
		}
		time.Sleep(350 * time.Millisecond)
	}

	if len(newTitles) > 0 {
		m.episodeTitlesCacheMu.Lock()
		for i, titles := range newTitles {
			m.episodeTitlesCache[i] = titles
		}
		m.episodeTitlesCacheMu.Unlock()
	}

}

func (m *SearchModel) fetchEpisodes(anime *source.Anime) tea.Cmd {
	translationType := m.searchState.TranslationType
	if translationType == "" {
		translationType = "sub"
	}
	return m.fetchEpisodesHianime(anime, translationType)
}

// fetchEpisodesHianime lists episodes from hianime. The provider also
// enriches anime.MALID from the embed URL in the background, which may
// arrive after this command completes - later titles fetches still benefit.
func (m *SearchModel) fetchEpisodesHianime(anime *source.Anime, translationType string) tea.Cmd {
	searchSelected := m.searchState.Selected
	hianimeProvider := m.hianimeProvider
	jikanClient := m.jikanClient

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()

		hianimeProvider.SetTranslation(translationType)
		eps, err := hianimeProvider.EpisodesOf(anime, 0)
		if err != nil {
			return EpisodesLoadedMsg{Episodes: nil, EpisodeTitles: nil, Error: err}
		}

		epList := make([]string, 0, len(eps))
		for _, ep := range eps {
			epList = append(epList, strconv.FormatFloat(ep.Number, 'f', -1, 64))
		}

		m.episodeTitlesCacheMu.Lock()
		var episodeTitles []string
		if titles, ok := m.episodeTitlesCache[searchSelected]; ok {
			episodeTitles = titles
		}
		m.episodeTitlesCacheMu.Unlock()
		// MALID may have been enriched by EpisodesOf just now.
		if len(episodeTitles) == 0 && anime.MALID > 0 {
			jikanEpisodes, err := jikanClient.GetEpisodes(ctx, anime.MALID)
			if err == nil && len(jikanEpisodes) > 0 {
				episodeTitles = make([]string, len(jikanEpisodes))
				for i, ep := range jikanEpisodes {
					episodeTitles[i] = ep.Title
				}
			}
		}

		return EpisodesLoadedMsg{Episodes: epList, EpisodeTitles: episodeTitles, Error: nil}
	}
}

func (m *SearchModel) fetchEpisodeMetadata() tea.Cmd {
	if len(m.episodeState.Episodes) == 0 || m.episodeState.Selected >= len(m.episodeState.Episodes) {
		m.episodeState.EpisodeMetadata = nil
		m.episodeState.MetadataLoading = false
		return nil
	}

	if len(m.searchState.Results) == 0 || m.searchState.Selected >= len(m.searchState.Results) {
		m.episodeState.EpisodeMetadata = nil
		m.episodeState.MetadataLoading = false
		return nil
	}

	anime := m.searchState.Results[m.searchState.Selected]
	if anime == nil || anime.MALID == 0 {
		m.episodeState.EpisodeMetadata = nil
		m.episodeState.MetadataLoading = false
		return nil
	}

	epNumStr := m.episodeState.Episodes[m.episodeState.Selected]
	cacheKey := fmt.Sprintf("%d-%s", anime.MALID, epNumStr)

	if cached, ok := m.episodeMetadataCache[cacheKey]; ok {
		m.episodeState.EpisodeMetadata = buildEpisodeMetadataPanel(cached)
		m.episodeState.MetadataLoading = false
		return nil
	}

	epNum, err := strconv.ParseFloat(epNumStr, 64)
	if err != nil {
		m.episodeState.EpisodeMetadata = nil
		m.episodeState.MetadataLoading = false
		return nil
	}

	idx := m.episodeState.Selected
	malID := anime.MALID
	jikanClient := m.jikanClient

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		episode, err := jikanClient.GetEpisode(ctx, malID, int(epNum))
		if err != nil {
			return EpisodeMetadataLoadedMsg{Metadata: nil, Index: idx}
		}

		return EpisodeMetadataLoadedMsg{Metadata: buildEpisodeMetadataPanel(episode), Index: idx, CacheKey: cacheKey, RawEpisode: episode}
	}
}

func buildEpisodeMetadataPanel(ep *jikan.Episode) *EpisodeMetadataPanel {
	return &EpisodeMetadataPanel{
		Title:         ep.Title,
		TitleJapanese: ep.TitleJapanese,
		Aired:         ep.Aired,
		Score:         ep.Score,
		Filler:        ep.Filler,
		Recap:         ep.Recap,
		Synopsis:      ep.Synopsis,
		Duration:      ep.Duration,
	}
}

func (m *SearchModel) playEpisode(anime *source.Anime, episodeNum string) tea.Cmd {
	translationType := m.searchState.TranslationType
	if translationType == "" {
		translationType = "sub"
	}
	hianimeProvider := m.hianimeProvider
	aniskipEnabled := m.settingsState.AniskipEnabled
	quality := m.settingsState.Quality
	animeTitle := ""
	if anime != nil {
		animeTitle = anime.Name
	}

	return func() tea.Msg {
		episodeNumFloat, _ := strconv.ParseFloat(episodeNum, 64)

		hianimeProvider.SetTranslation(translationType)
		streams, err := hianimeProvider.StreamsOf(&source.Episode{Number: episodeNumFloat, Anime: anime})
		if err != nil {
			return TUIErrorMsg{Err: fmt.Errorf("failed to get streams: %w", err)}
		}

		if len(streams) == 0 {
			return TUIErrorMsg{Err: fmt.Errorf("no streams found")}
		}

		var skipTimes []aniskip.SkipInterval
		if aniskipEnabled && anime != nil && anime.MALID > 0 {
			epNum, _ := strconv.Atoi(episodeNum)
			if times, err := aniskip.GetSkipTimes(anime.MALID, epNum); err == nil {
				skipTimes = times
			}
		}

		playStream := tryPlayStream(streams, animeTitle, episodeNum, skipTimes, quality)
		if playStream == nil {
			return TUIErrorMsg{Err: fmt.Errorf("no playable stream found")}
		}

		return PlayStreamMsg{}
	}
}

// updateTrackingCmd fires a fire-and-forget tracking update after playback starts.
func (m *SearchModel) updateTrackingCmd(anilistID, episodeNum, totalEpisodes int) tea.Cmd {
	token := m.anilistToken
	if token == "" || anilistID == 0 {
		return nil
	}

	return func() tea.Msg {
		client := anilist.NewAuthenticatedClient(token)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		status := "CURRENT"
		if totalEpisodes > 0 && episodeNum >= totalEpisodes {
			status = "COMPLETED"
		}

		err := client.SaveMediaListEntry(ctx, anilistID, status, episodeNum)
		return TrackingUpdateMsg{Status: status, Progress: episodeNum, Err: err}
	}
}

// fetchTrackingStatusCmd fetches the current AniList tracking status for an anime.
func (m *SearchModel) fetchTrackingStatusCmd(anilistID int) tea.Cmd {
	token := m.anilistToken
	if token == "" || anilistID == 0 {
		return nil
	}

	return func() tea.Msg {
		client := anilist.NewAuthenticatedClient(token)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		entry, err := client.GetMediaListStatus(ctx, anilistID)
		if err != nil {
			return TrackingStatusLoadedMsg{}
		}
		if entry == nil {
			return TrackingStatusLoadedMsg{}
		}
		return TrackingStatusLoadedMsg{Status: entry.Status, Progress: entry.Progress}
	}
}

func filterByQuality(streams []*source.Stream, quality string) []*source.Stream {
	if quality == "" || quality == "auto" {
		return streams
	}

	if quality == "best" {
		return streams
	}

	target := parseQualityNum(quality)

	var exact []*source.Stream
	for _, s := range streams {
		if s.Quality == quality {
			exact = append(exact, s)
		}
	}
	if len(exact) > 0 {
		return exact
	}

	var closest []*source.Stream
	bestDiff := int(^uint(0) >> 1)
	for _, s := range streams {
		q := parseQualityNum(s.Quality)
		diff := q - target
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			bestDiff = diff
			closest = []*source.Stream{s}
		} else if diff == bestDiff {
			closest = append(closest, s)
		}
	}
	return closest
}

func parseQualityNum(q string) int {
	q = strings.TrimSuffix(q, "p")
	n, _ := strconv.Atoi(q)
	if n == 0 {
		return 9999
	}
	return n
}

func tryPlayStream(streams []*source.Stream, animeTitle, episodeNum string, skipTimes []aniskip.SkipInterval, quality string) *source.Stream {
	d := &player.Detector{}

	filtered := filterByQuality(streams, quality)
	if len(filtered) == 0 {
		filtered = streams
	}

	ordered := make([]*source.Stream, len(filtered))
	copy(ordered, filtered)
	if player.IsAndroid() {
		sort.SliceStable(ordered, func(i, j int) bool {
			return !ordered[i].NeedsReferrer && ordered[j].NeedsReferrer
		})
	}

	var playerSkips []player.SkipInterval
	for _, st := range skipTimes {
		playerSkips = append(playerSkips, player.SkipInterval{
			Start: st.StartTime,
			End:   st.EndTime,
			Type:  st.Type,
		})
	}

	for _, s := range ordered {
		url := s.URL
		if strings.HasPrefix(url, "//") {
			url = "https:" + url
		}

		p := d.PreferredForReferrer(s.NeedsReferrer)
		opts := player.Options{
			Title:     fmt.Sprintf("%s - Episode %s", animeTitle, episodeNum),
			Referrer:  s.Referer,
			SkipTimes: playerSkips,
		}
		for _, sub := range s.Subtitles {
			opts.Subtitles = append(opts.Subtitles, sub.URL)
		}

		if err := p.Launch(url, opts); err == nil {
			return s
		}
	}

	return nil
}

// doAniListLogin starts the OAuth browser flow and returns a message when complete.
func doAniListLogin() tea.Cmd {
	return func() tea.Msg {
		auth.Quiet = true
		defer func() { auth.Quiet = false }()
		err := auth.Login()
		return AniListLoginMsg{Err: err}
	}
}

// doAniListLogout clears stored AniList credentials.
func doAniListLogout() tea.Cmd {
	return func() tea.Msg {
		auth.Quiet = true
		defer func() { auth.Quiet = false }()
		err := auth.Logout()
		return AniListLoginMsg{Err: err}
	}
}

// checkForUpdateCmd checks GitHub for the latest release and returns the version.
func checkForUpdateCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		release, err := update.CheckForUpdate(ctx)
		if err != nil {
			return UpdateCheckMsg{Err: err}
		}
		return UpdateCheckMsg{LatestVersion: release.TagName}
	}
}

// doSelfUpdateCmd downloads and installs the latest release.
func doSelfUpdateCmd(latestVersion string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		release, err := update.CheckForUpdate(ctx)
		if err != nil {
			return UpdateCompleteMsg{Err: fmt.Errorf("failed to fetch release: %w", err)}
		}

		if err := update.PerformUpdate(ctx, release); err != nil {
			return UpdateCompleteMsg{Err: err}
		}
		return UpdateCompleteMsg{}
	}
}

func matchScore(name, query string) int {
	if name == query {
		return 4
	}
	if strings.HasPrefix(name, query) {
		return 3
	}
	if strings.Contains(name, query) {
		return 2
	}
	queryWords := strings.Fields(query)
	if len(queryWords) > 1 {
		allMatch := true
		for _, w := range queryWords {
			if !strings.Contains(name, w) {
				allMatch = false
				break
			}
		}
		if allMatch {
			return 1
		}
	}
	return 0
}

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func stripHTML(s string) string {
	s = strings.ReplaceAll(s, "<br>", "\n")
	s = strings.ReplaceAll(s, "<br/>", "\n")
	s = strings.ReplaceAll(s, "<br />", "\n")
	return htmlTagRe.ReplaceAllString(s, "")
}

func truncateSynopsis(s string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	cut := string(runes[:maxLen])
	lastSpace := strings.LastIndex(cut, " ")
	if lastSpace == -1 {
		return cut + "..."
	}
	return cut[:lastSpace] + "..."
}
