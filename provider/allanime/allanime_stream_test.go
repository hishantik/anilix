package Allanime

import "testing"

func TestMP4UploadEmbedIsEligibleForExtractor(t *testing.T) {
	if isEmbedURL("https://mp4upload.com/embed-eb9zbmmnrgxp.html") {
		t.Fatal("MP4Upload embed must reach its registered extractor")
	}
}

func TestParseClockLinksSupportsLinksArray(t *testing.T) {
	body := []byte(`{"links":[{"link":"https://video.example/master.m3u8","hls":true,"resolutionStr":"1080"},{"link":"https://video.example/video.mp4","resolutionStr":"720"}]}`)
	streams, err := parseClockLinks(body, "uv-mp4")
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 2 || streams[0].URL != "https://video.example/master.m3u8" || streams[0].Quality != "1080p" || streams[1].Quality != "720p" {
		t.Fatalf("unexpected streams: %+v", streams)
	}
}
