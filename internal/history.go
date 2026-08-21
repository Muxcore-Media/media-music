package internal

import (
	"context"

	mediaadminv1 "github.com/Muxcore-Media/contracts-media-admin/gen/muxcore/media/admin/v1"
)

func (m *Module) ListHistory(ctx context.Context, req *mediaadminv1.ListHistoryRequest) (*mediaadminv1.ListHistoryResponse, error) {
	if m.store == nil {
		return nil, errNotInit
	}
	page := int(req.GetPage())
	if page < 1 {
		page = 1
	}
	pageSize := int(req.GetPageSize())
	if pageSize < 1 {
		pageSize = 20
	}
	rows, total, err := m.store.scanHistory(page, pageSize, req.GetItemId(), req.GetEventType())
	if err != nil {
		return nil, err
	}
	records := make([]*mediaadminv1.HistoryRecord, 0, len(rows))
	for _, r := range rows {
		records = append(records, &mediaadminv1.HistoryRecord{
			ItemId: r["item_id"], EventType: r["event_type"],
			SourceTitle: r["source_title"], Quality: r["quality"], CreatedAt: r["date"],
		})
	}
	return &mediaadminv1.ListHistoryResponse{
		Records: records, Total: int32(total), Page: int32(page), PageSize: int32(pageSize),
	}, nil
}

var errNotInit = errString("not initialized")

type errString string

func (e errString) Error() string { return string(e) }
