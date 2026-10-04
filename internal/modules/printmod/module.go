// Package printmod is the FSSP executive-production PDF task.
package printmod

import (
	"context"
	"errors"
	"strings"

	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/modules"
	"github.com/EldHasp/cursor-global-context/apps/agaeva-id/internal/print"
)

// Module prints HTML that the Bitrix24 tab already received from FSSP.
type Module struct {
	Styles  print.StylesheetSource
	Printer func(ctx context.Context, browserPath, document string) ([]byte, error)
	Find    func() (string, string, error)
}

func (m Module) Name() string  { return "print" }
func (m Module) Route() string { return "/print" }

func (m Module) Execute(ctx context.Context, req modules.Request) (modules.Response, error) {
	htmlDoc := strings.TrimSpace(req.HTML)
	if htmlDoc == "" {
		return modules.Response{}, errors.New("в запросе нет поля html")
	}
	var links []string
	if m.Styles != nil {
		got, err := m.Styles.Links(ctx)
		if err != nil && !print.HasStylesheet(htmlDoc) {
			return modules.Response{}, errors.New("не удалось взять стили с сайта ФССП, PDF не создан")
		}
		links = got
	} else if !print.HasStylesheet(htmlDoc) {
		return modules.Response{}, errors.New("не удалось взять стили с сайта ФССП, PDF не создан")
	}
	prepared, err := print.Prepare(htmlDoc, links)
	if err != nil {
		return modules.Response{}, err
	}
	find := m.Find
	if find == nil {
		find = print.FindBrowser
	}
	browserPath, _, err := find()
	if err != nil {
		return modules.Response{}, err
	}
	printer := m.Printer
	if printer == nil {
		printer = print.PrintHTML
	}
	pdf, err := printer(ctx, browserPath, prepared)
	if err != nil {
		return modules.Response{}, err
	}
	return modules.Response{
		Status:      200,
		ContentType: "application/pdf",
		Body:        pdf,
	}, nil
}
