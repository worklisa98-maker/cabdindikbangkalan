package services

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	stddraw "image/draw"
	_ "image/jpeg"
	"image/png"
	"math"
	"net/url"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/jung-kurt/gofpdf"
	xdraw "golang.org/x/image/draw"
)

type PDFReportRenderer struct {
	FrontendOrigin       string
	ChromeExecutablePath string
}

type PDFReportInput struct {
	Path        string
	Tab         string
	Query       url.Values
	Token       string
	FooterText  string
	PaperWidth  float64
	PaperHeight float64
	UserDataDir string
}

const (
	reportViewportWidth        int64 = 1800
	reportViewportHeight       int64 = 2200
	reportTargetImageWidth           = 1800
	reportRenderTimeoutPerPage       = 90 * time.Second
	reportScrollStepPx               = 900
	reportScrollTick                 = 100 * time.Millisecond
	reportStableDuration             = 800 * time.Millisecond
	reportMaxScrollChecks            = 500
	reportScreenshotQuality          = 90
	reportPostReadyDelay             = 400 * time.Millisecond
	reportChartWaitMaxChecks         = 20
	reportChartWaitTick              = 250 * time.Millisecond
	reportOverlapPx                  = 30
	reportMarginMM                   = 8.0
	reportFooterSpaceMM              = 8.0
	reportPageWMM                    = 297.0
	reportPageHMM                    = 210.0
)

type reportScreenshotPage struct {
	image         image.Image
	withWatermark bool
}

func (r PDFReportRenderer) Render(ctx context.Context, input PDFReportInput) ([]byte, error) {
	return r.RenderScreenshotPDF(ctx, input)
}

func (r PDFReportRenderer) RenderScreenshotPDF(ctx context.Context, input PDFReportInput) ([]byte, error) {
	return r.RenderScreenshotPDFMulti(ctx, []PDFReportInput{input})
}

func (r PDFReportRenderer) RenderScreenshotPDFMulti(
	ctx context.Context,
	inputs []PDFReportInput,
) ([]byte, error) {
	if len(inputs) == 0 {
		return nil, errors.New("at least one report input is required")
	}

	origin := strings.TrimSpace(r.FrontendOrigin)
	if origin == "" {
		return nil, errors.New("frontend origin is required")
	}
	if err := validateFrontendOrigin(origin); err != nil {
		return nil, err
	}
	for _, input := range inputs {
		if strings.TrimSpace(input.Path) == "" {
			return nil, errors.New("report path is required")
		}
		if strings.TrimSpace(input.Token) == "" {
			return nil, errors.New("report token is required")
		}
	}

	tmpDir, err := os.MkdirTemp("", "sipres-pdf-*")
	if err != nil {
		return nil, fmt.Errorf("create temp profile dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.WindowSize(int(reportViewportWidth), int(reportViewportHeight)),
		chromedp.UserDataDir(tmpDir),
	)
	execPath := resolveChromeExecPath(strings.TrimSpace(r.ChromeExecutablePath))
	if execPath != "" {
		opts = append(opts, chromedp.ExecPath(execPath))
	}

	allocCtx, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()

	taskCtx, cancelTask := chromedp.NewContext(allocCtx)
	defer cancelTask()

	timeout := reportRenderTimeoutPerPage * time.Duration(len(inputs))
	if timeout < reportRenderTimeoutPerPage {
		timeout = reportRenderTimeoutPerPage
	}
	taskCtx, cancelTimeout := context.WithTimeout(taskCtx, timeout)
	defer cancelTimeout()

	if err := chromedp.Run(taskCtx,
		network.Enable(),
		setReportTokenCookie(origin, inputs[0].Token),
		chromedp.EmulateViewport(reportViewportWidth, reportViewportHeight),
	); err != nil {
		return nil, fmt.Errorf("prepare report renderer: %w", err)
	}

	screenshots := make([]reportScreenshotPage, 0, len(inputs))
	for _, input := range inputs {
		reportURL, err := buildReportURL(origin, input)
		if err != nil {
			return nil, err
		}

		screenshotImage, err := captureReportScreenshot(taskCtx, reportURL)
		if err != nil {
			return nil, fmt.Errorf("render report page %s: %w", reportURL, err)
		}
		screenshots = append(screenshots, reportScreenshotPage{
			image:         screenshotImage,
			withWatermark: isWatermarkEnabled(reportURL),
		})
	}

	generatedAt := defaultReportGeneratedAt(time.Now())
	return renderScreenshotSlicesToPDF(screenshots, generatedAt)
}

func captureReportScreenshot(ctx context.Context, reportURL string) (image.Image, error) {
	if err := chromedp.Run(ctx,
		chromedp.Navigate(reportURL),
		waitDocumentComplete(),
		chromedp.WaitVisible("#report-root", chromedp.ByID),
		waitReportReady(),
		waitChartsRendered(),
		chromedp.Sleep(reportPostReadyDelay),
		forceWhiteBackground(),
		scrollPageUntilStable(),
	); err != nil {
		return nil, err
	}

	var screenshot []byte
	if err := chromedp.Run(ctx, chromedp.FullScreenshot(&screenshot, reportScreenshotQuality)); err != nil {
		return nil, fmt.Errorf("capture screenshot: %w", err)
	}
	screenshotImage, _, err := image.Decode(bytes.NewReader(screenshot))
	if err != nil {
		return nil, fmt.Errorf("decode screenshot image: %w", err)
	}
	return resizeImageToWidth(screenshotImage, reportTargetImageWidth), nil
}

func setReportTokenCookie(origin string, token string) chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		err := network.SetCookie("cabdin_token", token).
			WithURL(origin).
			WithPath("/").
			Do(ctx)
		return err
	})
}

func buildReportURL(origin string, input PDFReportInput) (string, error) {
	baseURL, err := url.Parse(strings.TrimSpace(origin))
	if err != nil {
		return "", err
	}
	reportRef, err := url.Parse(strings.TrimSpace(input.Path))
	if err != nil {
		return "", err
	}
	reportURL := baseURL.ResolveReference(reportRef)
	query := reportURL.Query()
	for key, values := range input.Query {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" || len(values) == 0 {
			continue
		}
		for i := len(values) - 1; i >= 0; i-- {
			value := strings.TrimSpace(values[i])
			if value == "" {
				continue
			}
			query.Set(trimmedKey, value)
			break
		}
	}
	if tab := strings.TrimSpace(input.Tab); tab != "" {
		query.Set("tab", tab)
	}
	query.Set("export", "1")
	reportURL.RawQuery = query.Encode()
	return reportURL.String(), nil
}

func validateFrontendOrigin(origin string) error {
	parsed, err := url.Parse(origin)
	if err != nil {
		return err
	}
	if !isProductionRuntime() {
		return nil
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	if host == "localhost" || host == "127.0.0.1" {
		return errors.New("FRONTEND_ORIGIN must not point to localhost in production")
	}
	return nil
}

func isProductionRuntime() bool {
	appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
	if appEnv == "production" {
		return true
	}
	for _, key := range []string{"ZEABUR", "ZEABUR_ENV", "ZEABUR_PROJECT_ID", "ZEABUR_SERVICE_ID"} {
		if strings.TrimSpace(os.Getenv(key)) != "" {
			return true
		}
	}
	return false
}

func resolveChromeExecPath(configPath string) string {
	if configPath != "" {
		return configPath
	}
	if runtime.GOOS == "linux" {
		if _, err := os.Stat("/usr/bin/chromium-browser"); err == nil {
			return "/usr/bin/chromium-browser"
		}
		if _, err := os.Stat("/usr/bin/chromium"); err == nil {
			return "/usr/bin/chromium"
		}
	}
	return ""
}

func waitDocumentComplete() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		for i := 0; i < 80; i++ {
			var ready bool
			err := chromedp.Evaluate(
				`(() => document.readyState === "complete")()`,
				&ready,
			).Do(ctx)
			if err == nil && ready {
				return nil
			}
			if err := sleepWithContext(ctx, 100*time.Millisecond); err != nil {
				return err
			}
		}
		return errors.New("document complete timeout")
	})
}

func waitReportReady() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		for i := 0; i < 240; i++ {
			var ready bool
			if err := chromedp.Evaluate(
				`(() => {
					const rootReady = document.querySelector("#report-root")?.getAttribute("data-report-ready")==="1";
					const htmlReady = document.documentElement?.dataset?.reportReady==="1";
					return Boolean(rootReady || htmlReady);
				})()`,
				&ready,
			).Do(ctx); err == nil && ready {
				return nil
			}

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(250 * time.Millisecond):
			}
		}
		return errors.New("report readiness timeout")
	})
}

func waitChartsRendered() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		for i := 0; i < reportChartWaitMaxChecks; i++ {
			var hasCharts bool
			if err := chromedp.Evaluate(
				`(() => document.querySelectorAll("svg, canvas").length > 0)()`,
				&hasCharts,
			).Do(ctx); err == nil && hasCharts {
				return nil
			}

			if err := sleepWithContext(ctx, reportChartWaitTick); err != nil {
				return err
			}
		}
		// Some pages (e.g. menu/list views) intentionally do not render charts.
		return nil
	})
}

func forceWhiteBackground() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		var ignored bool
		return chromedp.Evaluate(`(() => {
			document.documentElement.style.background = "#ffffff";
			if (document.body) {
				document.body.style.background = "#ffffff";
			}
			document.documentElement.style.colorScheme = "light";
			return true;
		})()`, &ignored).Do(ctx)
	})
}

func scrollPageUntilStable() chromedp.Action {
	return chromedp.ActionFunc(func(ctx context.Context) error {
		type scrollSnapshot struct {
			ScrollY      float64 `json:"scrollY"`
			InnerHeight  float64 `json:"innerHeight"`
			ScrollHeight float64 `json:"scrollHeight"`
		}

		var (
			lastHeight float64
			stableFor  time.Duration
		)
		for i := 0; i < reportMaxScrollChecks; i++ {
			var snapshot scrollSnapshot
			err := chromedp.Evaluate(`(() => ({
				scrollY: window.scrollY || window.pageYOffset || 0,
				innerHeight: window.innerHeight || document.documentElement.clientHeight || 0,
				scrollHeight: Math.max(
					document.documentElement.scrollHeight || 0,
					document.body ? document.body.scrollHeight : 0
				)
			}))()`, &snapshot).Do(ctx)
			if err != nil {
				return err
			}
			if snapshot.ScrollHeight == lastHeight {
				stableFor += reportScrollTick
			} else {
				stableFor = 0
				lastHeight = snapshot.ScrollHeight
			}
			atBottom := snapshot.ScrollY+snapshot.InnerHeight >= snapshot.ScrollHeight-2
			if atBottom && stableFor >= reportStableDuration {
				return nil
			}

			var ignored bool
			if err := chromedp.Evaluate(fmt.Sprintf(`(() => {
				window.scrollBy(0, %d);
				return true;
			})()`, reportScrollStepPx), &ignored).Do(ctx); err != nil {
				return err
			}
			if err := sleepWithContext(ctx, reportScrollTick); err != nil {
				return err
			}
		}
		return nil
	})
}

func resizeImageToWidth(src image.Image, targetWidth int) image.Image {
	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()
	if srcW <= 0 || srcH <= 0 || targetWidth <= 0 || srcW == targetWidth {
		return src
	}

	targetHeight := int(math.Round(float64(srcH) * float64(targetWidth) / float64(srcW)))
	if targetHeight <= 0 {
		return src
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), src, bounds, xdraw.Over, nil)
	return dst
}

type sliceRange struct {
	startY int
	endY   int
}

type screenshotSlice struct {
	src           image.Image
	startY        int
	endY          int
	withWatermark bool
}

func renderScreenshotSlicesToPDF(
	pages []reportScreenshotPage,
	generatedAt string,
) ([]byte, error) {
	if len(pages) == 0 {
		return nil, errors.New("no screenshots to render")
	}

	slices := make([]screenshotSlice, 0, len(pages)*2)
	usableWMM := reportPageWMM - (2 * reportMarginMM)
	for _, page := range pages {
		pageSlices, err := buildSliceRanges(page.image)
		if err != nil {
			return nil, err
		}
		for _, pageSlice := range pageSlices {
			slices = append(slices, screenshotSlice{
				src:           page.image,
				startY:        pageSlice.startY,
				endY:          pageSlice.endY,
				withWatermark: page.withWatermark,
			})
		}
	}
	if len(slices) == 0 {
		return nil, errors.New("no screenshot slices to render")
	}

	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(reportMarginMM, reportMarginMM, reportMarginMM)
	pdf.SetAutoPageBreak(false, reportMarginMM)

	totalPages := len(slices)
	for index, slice := range slices {
		pageImage := cropImage(slice.src, slice.startY, slice.endY)
		imgBuf := bytes.NewBuffer(nil)
		if err := png.Encode(imgBuf, pageImage); err != nil {
			return nil, fmt.Errorf("encode page image: %w", err)
		}

		pdf.AddPage()
		imageName := fmt.Sprintf("page-image-%d", index+1)
		imageOpts := gofpdf.ImageOptions{ImageType: "PNG"}
		pdf.RegisterImageOptionsReader(imageName, imageOpts, bytes.NewReader(imgBuf.Bytes()))
		pdf.ImageOptions(
			imageName,
			reportMarginMM,
			reportMarginMM,
			usableWMM,
			0,
			false,
			imageOpts,
			0,
			"",
		)

		if slice.withWatermark {
			drawCornerWatermark(pdf)
		}
		drawFooterText(pdf, generatedAt, index+1, totalPages)
	}

	out := bytes.NewBuffer(nil)
	if err := pdf.Output(out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func buildSliceRanges(src image.Image) ([]sliceRange, error) {
	bounds := src.Bounds()
	imgWidthPx := bounds.Dx()
	imgHeightPx := bounds.Dy()
	if imgWidthPx <= 0 || imgHeightPx <= 0 {
		return nil, errors.New("invalid screenshot size")
	}

	usableWMM := reportPageWMM - (2 * reportMarginMM)
	usableHMM := reportPageHMM - (2 * reportMarginMM) - reportFooterSpaceMM
	scaleMmPerPx := usableWMM / float64(imgWidthPx)
	maxHeightPxPerPage := int(math.Floor(usableHMM / scaleMmPerPx))
	if maxHeightPxPerPage <= 0 {
		return nil, errors.New("invalid page/image scaling")
	}

	slices := make([]sliceRange, 0, 2)
	if imgHeightPx <= maxHeightPxPerPage {
		slices = append(slices, sliceRange{startY: 0, endY: imgHeightPx})
		return slices, nil
	}

	firstEnd := maxHeightPxPerPage
	if firstEnd > imgHeightPx {
		firstEnd = imgHeightPx
	}
	slices = append(slices, sliceRange{startY: 0, endY: firstEnd})

	secondStart := firstEnd - reportOverlapPx
	if secondStart < 0 {
		secondStart = 0
	}
	secondEnd := secondStart + maxHeightPxPerPage
	if secondEnd > imgHeightPx {
		secondEnd = imgHeightPx
	}
	if secondStart < secondEnd {
		slices = append(slices, sliceRange{startY: secondStart, endY: secondEnd})
	}
	if len(slices) > 2 {
		slices = slices[:2]
	}
	return slices, nil
}

func cropImage(src image.Image, startY, endY int) image.Image {
	bounds := src.Bounds()
	if startY < 0 {
		startY = 0
	}
	if endY > bounds.Dy() {
		endY = bounds.Dy()
	}
	if endY <= startY {
		endY = startY + 1
	}

	rect := image.Rect(0, 0, bounds.Dx(), endY-startY)
	dst := image.NewRGBA(rect)
	srcPoint := image.Point{X: bounds.Min.X, Y: bounds.Min.Y + startY}
	stddraw.Draw(dst, rect, src, srcPoint, stddraw.Src)
	return dst
}

func drawFooterText(pdf *gofpdf.Fpdf, generatedAt string, pageNumber int, totalPages int) {
	footerText := fmt.Sprintf(
		"SIPRES Cabdin Bangkalan | Generated at %s | Page %d/%d",
		generatedAt,
		pageNumber,
		totalPages,
	)
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(82, 82, 82)
	pdf.SetXY(reportMarginMM, reportPageHMM-reportMarginMM-5)
	pdf.CellFormat(reportPageWMM-(2*reportMarginMM), 5, footerText, "", 0, "C", false, 0, "")
}

func drawCornerWatermark(pdf *gofpdf.Fpdf) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(176, 176, 176)
	pdf.SetXY(reportPageWMM-reportMarginMM-34, reportMarginMM+1)
	pdf.CellFormat(34, 5, "SIPRES", "", 0, "R", false, 0, "")
}

func defaultReportGeneratedAt(now time.Time) string {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.Local
	}
	return now.In(loc).Format("02 Jan 2006 15:04 WIB")
}

func isWatermarkEnabled(reportURL string) bool {
	if strings.TrimSpace(os.Getenv("REPORT_PDF_WATERMARK")) == "1" {
		return true
	}
	parsed, err := url.Parse(reportURL)
	if err != nil {
		return false
	}
	return strings.TrimSpace(parsed.Query().Get("watermark")) == "1"
}

func sleepWithContext(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
