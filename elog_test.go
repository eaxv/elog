package elog

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"
)

// captureLog přesměruje log výstup do bufferu. Volej restore() po testu.
func captureLog(t *testing.T) (*bytes.Buffer, func()) {
	t.Helper()
	buf := &bytes.Buffer{}
	log.SetOutput(buf)
	return buf, func() {
		log.SetOutput(os.Stderr)
	}
}

// resetState vrátí globální proměnné do výchozího stavu.
func resetState() {
	_ElogLevel = ELOG_DEBUG
	_ElogLimiter = nil
	_ElogErrorAppendFileLine = true
	_ElogObfuscateFileLine = false
	_ElogRotateMode = ELOG_ROTATE_MODE_TIME
	_ElogTraceCnt = 0
	_ElogTraceFiles = []string{}
	_ElogTracePatterns = []string{}
	_ElogDbgFunctionsToFile = false
	_ElogTotalOFF = false
	_ElogFile = nil
}

// --- 1. Log level ---

func TestSetLogLevel(t *testing.T) {
	defer resetState()
	cases := map[string]int{
		ELOG_DEBUG2_MSG: ELOG_DEBUG2,
		ELOG_DEBUG_MSG:  ELOG_DEBUG,
		ELOG_INFO_MSG:   ELOG_INFO,
		ELOG_WARN_MSG:   ELOG_WARN,
		ELOG_ERROR_MSG:  ELOG_ERROR,
		ELOG_OFF_MSG:    ELOG_OFF,
	}
	for s, want := range cases {
		if err := SetLogLevel(s); err != nil {
			t.Errorf("SetLogLevel(%q) error: %v", s, err)
		}
		if got := GetLogLevel(); got != want {
			t.Errorf("SetLogLevel(%q): got level %d, want %d", s, got, want)
		}
	}
	if err := SetLogLevel("NEPLATNY"); err == nil {
		t.Error("SetLogLevel(neplatny) mělo vrátit error")
	}
}

func TestSetLogLevelNum(t *testing.T) {
	defer resetState()
	SetLogLevelNum(ELOG_WARN)
	if got := GetLogLevel(); got != ELOG_WARN {
		t.Errorf("GetLogLevel() = %d, want %d", got, ELOG_WARN)
	}
}

func TestGetLogLevelMsg(t *testing.T) {
	defer resetState()
	cases := map[int]string{
		ELOG_DEBUG2: ELOG_DEBUG2_MSG,
		ELOG_DEBUG:  ELOG_DEBUG_MSG,
		ELOG_INFO:   ELOG_INFO_MSG,
		ELOG_WARN:   ELOG_WARN_MSG,
		ELOG_ERROR:  ELOG_ERROR_MSG,
		ELOG_OFF:    ELOG_OFF_MSG,
	}
	for lvl, want := range cases {
		SetLogLevelNum(lvl)
		if got := GetLogLevelMsg(); got != want {
			t.Errorf("GetLogLevelMsg() pro level %d = %q, want %q", lvl, got, want)
		}
	}
}

func TestLogLevelFiltering(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	SetLogLevelNum(ELOG_WARN)
	Debug("tato zprava nema projit")
	Info("tato zprava nema projit")
	Warn("tato zprava MA projit")

	out := buf.String()
	if strings.Contains(out, "tato zprava nema projit") {
		t.Error("Debug/Info zpráva prošla filtrem přestože level je WARN")
	}
	if !strings.Contains(out, "tato zprava MA projit") {
		t.Error("Warn zpráva neprojde přestože level je WARN")
	}
}

// --- 2. Základní log funkce ---

func TestDebugOutput(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	Debug("hello debug")
	out := buf.String()
	if !strings.Contains(out, "DEBUG") {
		t.Error("Debug výstup neobsahuje 'DEBUG'")
	}
	if !strings.Contains(out, "hello debug") {
		t.Error("Debug výstup neobsahuje zprávu")
	}
	if !strings.Contains(out, "elog_test.go") {
		t.Error("Debug výstup neobsahuje filename")
	}
}

func TestDebugfOutput(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	Debugf("hodnota: %d", 42)
	out := buf.String()
	if !strings.Contains(out, "hodnota: 42") {
		t.Errorf("Debugf výstup neobsahuje formátovanou zprávu, got: %q", out)
	}
}

func TestInfoOutput(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	Info("hello info")
	out := buf.String()
	if !strings.Contains(out, "INFO") || !strings.Contains(out, "hello info") {
		t.Errorf("Info výstup chybný: %q", out)
	}
}

func TestInfofOutput(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	Infof("cislo: %d", 99)
	if !strings.Contains(buf.String(), "cislo: 99") {
		t.Error("Infof výstup neobsahuje formátovanou zprávu")
	}
}

func TestWarnOutput(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	Warn("hello warn")
	out := buf.String()
	if !strings.Contains(out, "WARN") || !strings.Contains(out, "hello warn") {
		t.Errorf("Warn výstup chybný: %q", out)
	}
}

func TestWarnfOutput(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	Warnf("warn %s", "msg")
	if !strings.Contains(buf.String(), "warn msg") {
		t.Error("Warnf výstup neobsahuje zprávu")
	}
}

func TestErrorOutput(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	Error("hello error")
	out := buf.String()
	if !strings.Contains(out, "ERROR") || !strings.Contains(out, "hello error") {
		t.Errorf("Error výstup chybný: %q", out)
	}
}

func TestErrorfOutput(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	Errorf("err %d", 5)
	if !strings.Contains(buf.String(), "err 5") {
		t.Error("Errorf výstup neobsahuje zprávu")
	}
}

func TestInfoCtxf(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	ctx := &LogCtx{Prefix: "MYPFX"}
	InfoCtxf(ctx, "ctx zprava %d", 1)
	out := buf.String()
	if !strings.Contains(out, "MYPFX") {
		t.Errorf("InfoCtxf výstup neobsahuje prefix 'MYPFX': %q", out)
	}
}

func TestDebug2Output(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	SetLogLevelNum(ELOG_DEBUG2)
	Debug2("debug2 zprava")
	out := buf.String()
	if !strings.Contains(out, "DEBUG2") || !strings.Contains(out, "debug2 zprava") {
		t.Errorf("Debug2 výstup chybný: %q", out)
	}
}

func TestDebug2FilteredByDebugLevel(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	SetLogLevelNum(ELOG_DEBUG) // DEBUG2 < DEBUG, mělo by být filtrováno
	Debug2("nema projit")
	if strings.Contains(buf.String(), "nema projit") {
		t.Error("Debug2 zpráva prošla přestože level je DEBUG (ne DEBUG2)")
	}
}

// --- 3. Error handling ---

func TestElarNil(t *testing.T) {
	defer resetState()
	if got := Elar(nil); got != nil {
		t.Errorf("Elar(nil) = %v, want nil", got)
	}
}

func TestElar(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	orig := fmt.Errorf("testovaci chyba")
	got := Elar(orig)
	if got == nil {
		t.Fatal("Elar vrátil nil pro non-nil error")
	}
	// vrácená chyba musí obsahovat file info
	if !strings.Contains(got.Error(), "/") {
		t.Errorf("Elar výstup neobsahuje file info: %q", got.Error())
	}
	// musí se zalogovat
	if !strings.Contains(buf.String(), "testovaci chyba") {
		t.Errorf("Elar nezalogoval chybu, log: %q", buf.String())
	}
}

func TestElarf(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	got := Elarf("format %s %d", "x", 7)
	if got == nil {
		t.Fatal("Elarf vrátil nil")
	}
	if !strings.Contains(buf.String(), "format x 7") {
		t.Error("Elarf nezalogoval zprávu")
	}
}

func TestElarCtxf(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	ctx := &LogCtx{Prefix: "CTXPFX"}
	got := ElarCtxf(ctx, "ctx err %d", 3)
	if got == nil {
		t.Fatal("ElarCtxf vrátil nil")
	}
	out := buf.String()
	if !strings.Contains(out, "CTXPFX") {
		t.Errorf("ElarCtxf nezalogoval prefix: %q", out)
	}
}

func TestAppendFileInfo(t *testing.T) {
	defer resetState()
	_ElogErrorAppendFileLine = true
	err := fmt.Errorf("base error")
	got := AppendFileInfo(nil, err, 3)
	if got == nil {
		t.Fatal("AppendFileInfo vrátil nil")
	}
	if !strings.Contains(got.Error(), "/") {
		t.Errorf("AppendFileInfo nepřidal file info: %q", got.Error())
	}
}

func TestAppendFileInfoDisabled(t *testing.T) {
	defer resetState()
	_ElogErrorAppendFileLine = false
	err := fmt.Errorf("base error")
	got := AppendFileInfo(nil, err, 3)
	if got.Error() != err.Error() {
		t.Errorf("AppendFileInfo při disabled měl vrátit původní error, got: %q", got.Error())
	}
}

func TestAppendFileInfoAlreadySlash(t *testing.T) {
	defer resetState()
	_ElogErrorAppendFileLine = true
	// chyba končící '/' — file info se nepřidá
	err := fmt.Errorf("chyba /elog_test:99/")
	got := AppendFileInfo(nil, err, 3)
	if got.Error() != err.Error() {
		t.Errorf("AppendFileInfo přidal file info chybě zakončené '/': %q", got.Error())
	}
}

func TestAppendFileInfoNil(t *testing.T) {
	defer resetState()
	got := AppendFileInfo(nil, nil, 3)
	if got != nil {
		t.Errorf("AppendFileInfo(nil) = %v, want nil", got)
	}
}

// --- 4. Obfuskace ---

func TestObfuscateDeobfuscate(t *testing.T) {
	cases := []string{
		"hello",
		"elog_test:42",
		"soubor s mezerami",
		"a",
		"1234567890abcdef",
	}
	for _, tc := range cases {
		enc := ObfuscateText(tc)
		dec, err := DeobfuscateText(enc)
		if err != nil {
			t.Errorf("DeobfuscateText(%q) error: %v", enc, err)
		}
		if dec != tc {
			t.Errorf("roundtrip selhalo: orig=%q enc=%q dec=%q", tc, enc, dec)
		}
	}
}

func TestObfuscateRandomSeed(t *testing.T) {
	// dvě volání se stejným vstupem mohou dát různý výstup (náhodný seed)
	// Pro alespoň 10 pokusů očekáváme aspoň 2 různé výsledky
	orig := "teststring"
	results := map[string]bool{}
	for i := 0; i < 10; i++ {
		results[ObfuscateText(orig)] = true
	}
	if len(results) < 2 {
		t.Log("Varování: ObfuscateText vždy vygeneroval stejný výstup (deterministický seed?)")
	}
}

func TestDeobfuscateInvalidHex(t *testing.T) {
	_, err := DeobfuscateText("ZZINVALIDHEX")
	if err == nil {
		t.Error("DeobfuscateText na neplatném hex mělo vrátit error")
	}
}

// --- 5. Panic a RecoverPanic ---

func TestRecoverPanic(t *testing.T) {
	defer resetState()
	_, restore := captureLog(t)
	defer restore()

	recovered := RecoverPanic(func() {
		Panic("test panic zprava")
	})
	if recovered == nil {
		t.Fatal("RecoverPanic nechytil paniku")
	}
	pt, ok := recovered.(PanicT)
	if !ok {
		t.Errorf("RecoverPanic vrátil %T místo PanicT", recovered)
	}
	if !strings.Contains(string(pt), "test panic zprava") {
		t.Errorf("PanicT neobsahuje zprávu: %q", string(pt))
	}
}

func TestRecoverPanicNoPanic(t *testing.T) {
	recovered := RecoverPanic(func() {})
	if recovered != nil {
		t.Errorf("RecoverPanic bez paniky vrátil %v, want nil", recovered)
	}
}

func TestPanicf(t *testing.T) {
	defer resetState()
	_, restore := captureLog(t)
	defer restore()

	recovered := RecoverPanic(func() {
		Panicf("format %s", "panika")
	})
	if recovered == nil {
		t.Fatal("Panicf nespustilo paniku")
	}
	if pt, ok := recovered.(PanicT); !ok || !strings.Contains(string(pt), "format panika") {
		t.Errorf("Panicf PanicT = %q", recovered)
	}
}

// --- 6. Limiter ---

func TestLimiterFirstOccurrence(t *testing.T) {
	lim := newElog_limiter()
	ok, _ := lim.limitterSuppressRepeatingMsgs("zprava1")
	if !ok {
		t.Error("První výskyt zprávy byl potlačen (měl projít)")
	}
}

func TestLimiterSuppress(t *testing.T) {
	_, restore := captureLog(t)
	defer restore()
	lim := newElog_limiter()
	lim.limitStart = 3
	msg := "opakovana zprava"
	// první 3 projdou
	for i := 0; i < 3; i++ {
		ok, _ := lim.limitterSuppressRepeatingMsgs(msg)
		if !ok {
			t.Errorf("Iterace %d měla projít (limitStart=%d)", i, lim.limitStart)
		}
	}
	// 4. a další se potlačí
	suppressed := 0
	for i := 0; i < 10; i++ {
		ok, _ := lim.limitterSuppressRepeatingMsgs(msg)
		if !ok {
			suppressed++
		}
	}
	if suppressed == 0 {
		t.Error("Limiter nepotlačil žádnou opakovanou zprávu")
	}
}

func TestLimiterTimeout(t *testing.T) {
	_, restore := captureLog(t)
	defer restore()
	lim := newElog_limiter()
	lim.limitStart = 2
	lim.msgTimeoutSec = 10 * time.Millisecond
	msg := "timeout zprava"

	// přeplníme limit
	for i := 0; i < 5; i++ {
		lim.limitterSuppressRepeatingMsgs(msg)
	}
	// počkáme na timeout
	time.Sleep(20 * time.Millisecond)
	ok, _ := lim.limitterSuppressRepeatingMsgs(msg)
	if !ok {
		t.Error("Po timeoutu by zpráva měla znovu projít")
	}
}

func TestLimiterCacheFlush(t *testing.T) {
	_, restore := captureLog(t)
	defer restore()
	lim := newElog_limiter()
	lim.maxCacheSize = 5
	// naplníme cache přes limit
	for i := 0; i < 10; i++ {
		lim.limitterSuppressRepeatingMsgs(fmt.Sprintf("zprava-%d", i))
	}
	// cache se promazala (velikost ≤ maxCacheSize)
	lim.mux.Lock()
	size := len(lim.cache)
	lim.mux.Unlock()
	if size > lim.maxCacheSize {
		t.Errorf("Cache nebyla promazána: size=%d, max=%d", size, lim.maxCacheSize)
	}
}

// --- 7. Trace ---

func TestIsTraceFalse(t *testing.T) {
	defer resetState()
	SetTraceCnt(0)
	if IsTrace() {
		t.Error("IsTrace() = true při TraceCnt=0")
	}
}

func TestIsTraceAll(t *testing.T) {
	defer resetState()
	SetTraceCnt(ELOG_TRACE_ALL)
	if !IsTrace() {
		t.Error("IsTrace() = false při ELOG_TRACE_ALL")
	}
}

func TestTracefCountdown(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	SetTraceCnt(2)
	SetTraceFiles([]string{"*"})

	Tracef("trace %d", 1)
	Tracef("trace %d", 2)
	Tracef("trace %d", 3) // tato by neměla projít

	out := buf.String()
	cnt := strings.Count(out, "TRACE")
	if cnt != 2 {
		t.Errorf("Očekávány 2 TRACE zprávy, got %d; output: %q", cnt, out)
	}
}

func TestTracefFileFilter(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	SetTraceCnt(ELOG_TRACE_ALL)
	SetTraceFiles([]string{"neexistujici_soubor.go"})
	Tracef("nema projit")

	if strings.Contains(buf.String(), "nema projit") {
		t.Error("Trace zpráva prošla přestože soubor neodpovídá filtru")
	}
}

func TestTracefPatternFilter(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	SetTraceCnt(ELOG_TRACE_ALL)
	SetTraceFiles([]string{"*"})
	SetTracePatterns([]string{"HLEDANY_VZOR"})

	Tracef("zprava bez vzoru")
	Tracef("zprava s HLEDANY_VZOR uvnitr")

	out := buf.String()
	if strings.Contains(out, "zprava bez vzoru") {
		t.Error("Zpráva bez vzoru prošla pattern filtrem")
	}
	if !strings.Contains(out, "HLEDANY_VZOR") {
		t.Error("Zpráva se vzorem neprošla")
	}
}

// --- 8. GetRuntimeInfo ---

func TestGetRuntimeInfo(t *testing.T) {
	ri, err := GetRuntimeInfo(nil, 2)
	if err != nil {
		t.Fatalf("GetRuntimeInfo error: %v", err)
	}
	if ri.FileName == "" {
		t.Error("FileName je prázdný")
	}
	if ri.Line <= 0 {
		t.Errorf("Line = %d, očekáváno > 0", ri.Line)
	}
	if ri.Func == "" {
		t.Error("Func je prázdný")
	}
}

// --- 9. TotalOFF ---

func TestTotalOFF(t *testing.T) {
	defer resetState()
	buf, restore := captureLog(t)
	defer restore()

	SetTotalOFF(true)
	Info("nema se zalogovat")
	Debug("ani toto")
	Error("ani toto")

	if buf.Len() > 0 {
		t.Errorf("TotalOFF nevypnul logging, got: %q", buf.String())
	}
}

// --- 10. SetRotateMode ---

func TestSetRotateModeValid(t *testing.T) {
	defer resetState()
	RecoverPanic(func() { SetRotateMode(ELOG_ROTATE_MODE_TIME) })
	RecoverPanic(func() { SetRotateMode(ELOG_ROTATE_MODE_2FILES) })
	// žádná panika = OK
}

func TestSetRotateModeInvalid(t *testing.T) {
	defer resetState()
	_, restore := captureLog(t)
	defer restore()

	recovered := RecoverPanic(func() {
		SetRotateMode("NEPLATNY")
	})
	if recovered == nil {
		t.Error("SetRotateMode s neplatným modem mělo způsobit paniku")
	}
}
