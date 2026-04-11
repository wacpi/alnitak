package service

import (
	"testing"

	"interastral-peace.com/alnitak/internal/domain/vo"
)

func TestMergeHistoryPGCRowsIntoVideos(t *testing.T) {
	videos := []vo.HistoryVideoResp{
		{ID: 1, PGCAttached: false, Title: "ugc"},
		{ID: 84, PGCAttached: true, Title: "upload"},
		{ID: 85, PGCAttached: true, Title: "orphan"},
	}
	rows := []historyPGCRow{
		{Vid: 84, EpID: 15, EpisodeNumber: 1, EpisodeTitle: "第一话", PGCTitle: "测试系列"},
		{Vid: 84, EpID: 14, EpisodeNumber: 0, EpisodeTitle: "旧绑定", PGCTitle: "应忽略"},
	}

	mergeHistoryPGCRowsIntoVideos(videos, rows)

	if videos[0].EpID != 0 || videos[0].PGCTitle != "" {
		t.Fatalf("非 PGC 行不应被写入: %+v", videos[0])
	}
	if videos[1].EpID != 15 || videos[1].PGCTitle != "测试系列" || videos[1].EpisodeTitle != "第一话" || videos[1].EpisodeNumber != 1 {
		t.Fatalf("PGC 行合并错误: %+v", videos[1])
	}
	if videos[2].EpID != 0 {
		t.Fatalf("无匹配剧集时应保持空: %+v", videos[2])
	}
}
