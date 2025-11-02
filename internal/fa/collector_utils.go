package fa

import (
	"github.com/PuerkitoBio/goquery"
	"github.com/senexdrake/furaffinity-notifier/internal/fa/entries"
)

const (
	removeHeaders           = true
	removeFooters           = true
	removeSubmissionHeaders = removeHeaders
	removeSubmissionFooters = removeFooters

	// Note: FA journals have a different content structure. They have a ".journal-content" element that only contains
	// the actual body of the content, without any headers or footers. So it's not necessary to try to remove these for now.
	removeJournalHeaders = false
	removeJournalFooters = false
)

func (fc *FurAffinityCollector) isRemoveHeaders(entryType entries.EntryType) bool {
	switch entryType {
	case entries.EntryTypeSubmission:
		return removeSubmissionHeaders
	case entries.EntryTypeJournal:
		return removeJournalHeaders
	default:
		return false
	}
}

func (fc *FurAffinityCollector) isRemoveFooters(entryType entries.EntryType) bool {
	switch entryType {
	case entries.EntryTypeSubmission:
		return removeSubmissionFooters
	case entries.EntryTypeJournal:
		return removeJournalFooters
	default:
		return false
	}
}

func (fc *FurAffinityCollector) removeHeaders(sel *goquery.Selection, entryType entries.EntryType) {
	if !fc.isRemoveHeaders(entryType) {
		return
	}
	selector := headerSelector(entryType)
	if selector == "" {
		return
	}

	sel.Find(selector).Remove()
}

func (fc *FurAffinityCollector) removeFooters(sel *goquery.Selection, entryType entries.EntryType) {
	if !fc.isRemoveFooters(entryType) {
		return
	}
	selector := footerSelector(entryType)
	if selector == "" {
		return
	}

	sel.Find(selector).Remove()
}

func (fc *FurAffinityCollector) removeHeadersAndFooters(sel *goquery.Selection, entryType entries.EntryType) {
	fc.removeHeaders(sel, entryType)
	fc.removeFooters(sel, entryType)
}

func headerSelector(entryType entries.EntryType) string {
	switch entryType {
	case entries.EntryTypeSubmission:
		return ".submission-header"
	case entries.EntryTypeJournal:
		return ".journal-header"
	default:
		return ""
	}
}

func footerSelector(entryType entries.EntryType) string {
	switch entryType {
	case entries.EntryTypeSubmission:
		return ".submission-footer"
	case entries.EntryTypeJournal:
		return ".journal-footer"
	default:
		return ""
	}
}
