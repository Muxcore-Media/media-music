package internal

import mediaadminv1 "github.com/Muxcore-Media/contracts-media-admin/gen/muxcore/media/admin/v1"

const (
	historyImport     = "import"
	historyDeleteFile = "delete_file"
	historyDeleteItem = "delete_item"
	historyGrab       = "grab"
)

func sortFieldToColumn(sf mediaadminv1.SortField) string {
	switch sf {
	case mediaadminv1.SortField_SORT_FIELD_CREATED_AT:
		return "created_at"
	case mediaadminv1.SortField_SORT_FIELD_UPDATED_AT:
		return "updated_at"
	case mediaadminv1.SortField_SORT_FIELD_TITLE:
		return "name"
	default:
		return "name"
	}
}

func historyEventFilter(et mediaadminv1.HistoryEventType) string {
	switch et {
	case mediaadminv1.HistoryEventType_HISTORY_EVENT_TYPE_GRAB:
		return historyGrab
	case mediaadminv1.HistoryEventType_HISTORY_EVENT_TYPE_IMPORT:
		return historyImport
	case mediaadminv1.HistoryEventType_HISTORY_EVENT_TYPE_DELETE_ITEM:
		return historyDeleteItem
	case mediaadminv1.HistoryEventType_HISTORY_EVENT_TYPE_DELETE_FILE:
		return historyDeleteFile
	default:
		return ""
	}
}

func storeEventToProto(ev string) mediaadminv1.HistoryEventType {
	switch ev {
	case historyGrab:
		return mediaadminv1.HistoryEventType_HISTORY_EVENT_TYPE_GRAB
	case historyImport, "artistAdded", "trackFileImported":
		return mediaadminv1.HistoryEventType_HISTORY_EVENT_TYPE_IMPORT
	case historyDeleteItem:
		return mediaadminv1.HistoryEventType_HISTORY_EVENT_TYPE_DELETE_ITEM
	case historyDeleteFile, "trackFileDeleted":
		return mediaadminv1.HistoryEventType_HISTORY_EVENT_TYPE_DELETE_FILE
	default:
		return mediaadminv1.HistoryEventType_HISTORY_EVENT_TYPE_UNSPECIFIED
	}
}
