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
)

func (m *SearchModel) doSearch(query string) tea.Cmd {
	translationType := m.searchState.TranslationType
	if translationType == "" {
		translationType = "sub"
	}
	allanimeClient := m.allanimeClient

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		shows, err := allanimeClient.SearchShows(ctx, query, 20, 1, translationType)
		if err != nil {
			return SearchErrorMsg{Err: err}
		}

		results := make([]*source.Anime, 0, len(shows))
		for _, show := range shows {
			anime := allanimeClient.MapToAnime(&show)
			results = append(results, anime)
		}

		queryLower := strings.ToLower(query)
		sort.SliceStable(results, func(i, j int) bool {
			nameI := strings.ToLower(results[i].Name)
			nameJ := strings.ToLower(results[j].Name)
			scoreI := matchScore(nameI, queryLower)
			scoreJ := matchScore(nameJ, queryLower)
			return scoreI > scoreJ
		})

		return SearchResultsMsg{Results: results}
	}
}

// fetchMetadataBatch fetches metadata for all results in a single AniList GraphQL call.
func (m *SearchModel) fetchMetadataBatch(results []*source.Anime) tea.Cmd {
	// Collect all AniList IDs with their result index
	type indexID struct {
		Index      int
		AniListID  int
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
func downloadCoverCmd(coverURL string, panelWidth int, index int, maxRows int) tea.Cmd {
	if coverURL == "" {
		return nil
	}
	protocol := DetectTerminalProtocol()
	if protocol == ProtocolNone {
		return nil
	}
	return func() tea.Msg {
		img, cachePath, err := DownloadCoverImage(coverURL)
		if err != nil {
			return CoverImageLoadedMsg{Rendered: "", Index: index}
		}
		result := RenderCoverImageKitty(img, panelWidth, protocol, cachePath, maxRows)
		return CoverImageLoadedMsg{
			Rendered:         result.Placeholder,
			KittyTransmitSeq: result.TransmitSeq,
			KittyImageID:     result.ImageID,
			Index:            index,
		}
	}
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

func (m *SearchModel) fetchEpisodes(showID string, malID int) tea.Cmd {
	translationType := m.searchState.TranslationType
	if translationType == "" {
		translationType = "sub"
	}
	searchSelected := m.searchState.Selected
	allanimeClient := m.allanimeClient
	jikanClient := m.jikanClient

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		episodes, err := allanimeClient.GetShowEpisodes(ctx, showID, translationType)
		if err != nil {
			return EpisodesLoadedMsg{Episodes: nil, EpisodeTitles: nil, Error: err}
		}

		epList, ok := episodes[translationType]
		if !ok {
			epList, ok = episodes["sub"]
			if !ok {
				return EpisodesLoadedMsg{Episodes: nil, EpisodeTitles: nil, Error: fmt.Errorf("no episodes found for %s", translationType)}
			}
		}

		m.episodeTitlesCacheMu.Lock()
		var episodeTitles []string
		if titles, ok := m.episodeTitlesCache[searchSelected]; ok {
			episodeTitles = titles
		}
		m.episodeTitlesCacheMu.Unlock()
		if len(episodeTitles) == 0 && malID > 0 {
			jikanEpisodes, err := jikanClient.GetEpisodes(ctx, malID)
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

func (m *SearchModel) playEpisode(showID, episodeNum, animeTitle string, malID int) tea.Cmd {
	translationType := m.searchState.TranslationType
	if translationType == "" {
		translationType = "sub"
	}
	allanimeProvider := m.allanimeProvider
	aniskipEnabled := m.settingsState.AniskipEnabled
	quality := m.settingsState.Quality

	return func() tea.Msg {
		allanimeProvider.SetTranslation(translationType)

		episodeNumFloat, _ := strconv.ParseFloat(episodeNum, 64)
		episode := &source.Episode{
			Number: episodeNumFloat,
			Anime: &source.Anime{
				AllAnimeID: showID,
				Name:       animeTitle,
			},
		}

		streams, err := allanimeProvider.StreamsOf(episode)
		if err != nil {
			return TUIErrorMsg{Err: fmt.Errorf("failed to get streams: %w", err)}
		}

		if len(streams) == 0 {
			return TUIErrorMsg{Err: fmt.Errorf("no streams found")}
		}

		var skipTimes []aniskip.SkipInterval
		if aniskipEnabled && malID > 0 {
			epNum, _ := strconv.Atoi(episodeNum)
			if times, err := aniskip.GetSkipTimes(malID, epNum); err == nil {
				skipTimes = times
			}
		}

		// Read saved playback position for resume
		posKey := player.MakePosKey(animeTitle, episodeNum)
		startPos := player.ReadPosition(posKey)

		playStream := tryPlayStream(streams, animeTitle, episodeNum, skipTimes, quality, startPos)
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

func tryPlayStream(streams []*source.Stream, animeTitle, episodeNum string, skipTimes []aniskip.SkipInterval, quality string, startPos float64) *source.Stream {
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

	posKey := player.MakePosKey(animeTitle, episodeNum)

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
			StartPos:  startPos,
			PosKey:    posKey,
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
	if len(s) <= maxLen {
		return s
	}
	lastSpace := strings.LastIndex(s[:maxLen], " ")
	if lastSpace == -1 {
		return s[:maxLen] + "..."
	}
	return s[:lastSpace] + "..."
}
