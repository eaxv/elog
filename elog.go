package elog

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"os"
	"reflect"
	"regexp"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/k0kubun/pp"
)

const ELOG_DEBUG2 = 6
const ELOG_DEBUG = 5
const ELOG_INFO = 4
const ELOG_WARN = 3
const ELOG_ERROR = 2

// const ELOG_FATAL = 1
const ELOG_PANIC = 0
const ELOG_OFF = -1

const ELOG_DEBUG2_MSG = "DEBUG2"
const ELOG_DEBUG_MSG = "DEBUG"
const ELOG_WARN_MSG = "WARN"
const ELOG_INFO_MSG = "INFO"
const ELOG_ERROR_MSG = "ERROR"
const ELOG_OFF_MSG = "OFF"

const ELOG_DBG_FUNCTION = "DBG_FUNC"

// const ELOG_FATAL_MSG = "FATAL"
const ELOG_PANIC_MSG = "PANIC"

const ELOG_TRACE_PREFIX = "TRACE"

const ELOG_ROTATE_MODE_TIME = "T"
const ELOG_ROTATE_MODE_2FILES = "2"

var _ElogLevel = ELOG_DEBUG
var _ElogLimiter *ElogLimiter = nil
var _ElogErrorAppendFileLine = true
var _ElogObfuscateFileLine = false
var _ElogRotateMode = ELOG_ROTATE_MODE_TIME

const ELOG_TRACE_ALL = -1000

var _ElogTraceCnt = 0
var _ElogTraceFiles = []string{}
var _ElogTracePatterns = []string{}

// presmerovani Dbg... funkci do souboru s level DEBUG
var _ElogDbgFunctionsToFile = false

// uplne vypne elog
// pouzivam kdyz chci pustit vsechny testy a nechci videt zadne hlasky
var _ElogTotalOFF = false

var _ElogFile *os.File

type PanicT string

// >> limiter opakujicich se hlasek
type ElogLimiterValT struct {
	cnt         int
	tm          time.Time
	limit_print int
}

type ElogLimiter struct {
	mux           sync.Mutex
	cache         map[string]ElogLimiterValT
	limitStart    int
	maxCacheSize  int
	msgTimeoutSec time.Duration
	prefix        string
}

func newElog_limiter() *ElogLimiter {
	lf := ElogLimiter{}
	lf.mux = sync.Mutex{}
	lf.cache = map[string]ElogLimiterValT{}
	lf.limitStart = 10
	lf.maxCacheSize = 1024
	lf.msgTimeoutSec = time.Second * 20
	lf.prefix = "ELOG-LIMITER"
	return &lf
}

func (self *ElogLimiter) Log(format string, o ...interface{}) {
	s := fmt.Sprintf(format, o...)
	ss := fmt.Sprintf("%s %s", self.prefix, s)
	log.Printf(ss)
}

// uklada hlasky do mapy a opakujici se hlasky potlacuje podle definovanych pravidel
// diky tomu je mozne i pouzivat DEBUG se zapnutym lua klientem
func (self *ElogLimiter) limitterSuppressRepeatingMsgs(msg string) (bool, string) {
	var found bool
	var val ElogLimiterValT

	self.mux.Lock()
	defer self.mux.Unlock()

	//cache dovolime max ...
	if len(self.cache) > self.maxCacheSize {
		self.Log("cache flushed - size: %d", len(self.cache))
		self.cache = map[string]ElogLimiterValT{}
	}
	now := time.Now()
	val, found = self.cache[msg]
	//pokud neni v cache
	if !found {
		val = ElogLimiterValT{1, now, self.limitStart}
		self.cache[msg] = val
		return true, msg
	}
	//pokud jsme hlasku nevideli X sec vypneme limitaci
	if now.Sub(val.tm) > self.msgTimeoutSec {
		self.Log("limit msg stopped - timeout cnt: %d msg: %s", val.cnt, msg)
		val.tm = now
		val.cnt = 1
		val.limit_print = self.limitStart
		self.cache[msg] = val
		return true, msg
	}
	//pro opakujici se hlasky
	if val.cnt >= self.limitStart {
		if val.cnt == self.limitStart {
			self.Log("limiting msg started : %s", msg)
		}
		val.tm = now
		val.cnt += 1
		self.cache[msg] = val
		if (val.cnt % val.limit_print) == 0 {
			val.limit_print *= 2
			self.cache[msg] = val
			return true, fmt.Sprintf("%s (%s: %d)", msg, self.prefix, val.cnt)
		}
		return false, msg
	}
	//zbytek
	val.tm = now
	val.cnt += 1
	self.cache[msg] = val
	return true, msg
}

// <<

func SetLimiter() {
	_ElogLimiter = newElog_limiter()
	_ElogLimiter.Log("ON cache: %d limit start: %d timeout: %v",
		_ElogLimiter.maxCacheSize, _ElogLimiter.limitStart, _ElogLimiter.msgTimeoutSec)
}

func SetTraceCnt(cnt int) {
	_ElogTraceCnt = cnt
}

func GetTraceCnt() int {
	return _ElogTraceCnt
}

func SetTraceFiles(flt []string) {
	_ElogTraceFiles = flt
}

func SetDbgFunctionToFile(b bool) {
	_ElogDbgFunctionsToFile = b
}

func SetTotalOFF(b bool) {
	_ElogTotalOFF = b
}

func GetTraceFiles() []string {
	return _ElogTraceFiles
}

func SetTracePatters(pat []string) {
	_ElogTracePatterns = pat
}

func GetTracePatterns() []string {
	return _ElogTracePatterns
}

func GetLogLevel() int {
	return _ElogLevel
}

func GetLogLevelMsg() string {

	switch GetLogLevel() {
	case ELOG_DEBUG2:
		return ELOG_DEBUG2_MSG
	case ELOG_DEBUG:
		return ELOG_DEBUG_MSG
	case ELOG_WARN:
		return ELOG_WARN_MSG
	case ELOG_INFO:
		return ELOG_INFO_MSG
	case ELOG_ERROR:
		return ELOG_ERROR_MSG
	case ELOG_OFF:
		return ELOG_OFF_MSG
	}
	return fmt.Sprintf("Error: Unknown log level: %s\n", GetLogLevel())
}

func SetLogLevelNum(level int) {
	_ElogLevel = level
}

func SetRotateMode(mode string) {
	if mode != ELOG_ROTATE_MODE_TIME && mode != ELOG_ROTATE_MODE_2FILES {
		Panicf("Invalid rotate mode: %s", mode)
	}
	_ElogRotateMode = mode
}

func SetErrorAppendFileLine(b bool) {
	_ElogErrorAppendFileLine = b
}

func SetObfuscateFileLine(b bool) {
	_ElogObfuscateFileLine = b
}

func SetLogLevel(level string) error {
	switch level {
	case ELOG_DEBUG2_MSG:
		_ElogLevel = ELOG_DEBUG2
	case ELOG_DEBUG_MSG:
		_ElogLevel = ELOG_DEBUG
	case ELOG_WARN_MSG:
		_ElogLevel = ELOG_WARN
	case ELOG_INFO_MSG:
		_ElogLevel = ELOG_INFO
	case ELOG_ERROR_MSG:
		_ElogLevel = ELOG_ERROR
	case ELOG_OFF_MSG:
		_ElogLevel = ELOG_OFF
	default:
		return fmt.Errorf("Error: Unknown log level: %s\n", level)
	}
	return nil
}

type LogCtx struct {
	Prefix    string
	StackSkip int
}

type RuntimeInfoT struct {
	FileName string
	Line     int
	Func     string
}

func GetRuntimeInfo(ctx *LogCtx, skip int) (RuntimeInfoT, error) {
	var ri RuntimeInfoT

	if ctx != nil {
		skip += ctx.StackSkip
	}

	fpcs := make([]uintptr, 1)
	n := runtime.Callers(skip, fpcs)
	if n == 0 {
		return ri, fmt.Errorf("GetRuntimeInfo n = 0")
	}
	caller := runtime.FuncForPC(fpcs[0] - 1)
	if caller == nil {
		//fmt.Println("MSG CALLER WAS NIL")
		return ri, fmt.Errorf("GetRuntimeInfo caller == nil")
	}
	file, ln := caller.FileLine(fpcs[0] - 1)
	aa := strings.Split(file, "/")
	if len(aa) < 1 {
		ri.FileName = file
	} else {
		ri.FileName = aa[len(aa)-1]
	}
	ri.Line = ln
	ri.Func = caller.Name()
	idx := strings.LastIndex(ri.Func, ".")
	if idx > 0 {
		ri.Func = ri.Func[idx+1:]
	}
	return ri, nil
}

func core_logit(ctx *LogCtx, o ...interface{}) {
	if _ElogTotalOFF {
		return
	}
	ss := fmt.Sprint(o...)
	if ctx != nil && ctx.Prefix != "" {
		ss = fmt.Sprintf("<%s> %s", ctx.Prefix, ss)
	}
	if _ElogLimiter != nil {
		b, ss2 := _ElogLimiter.limitterSuppressRepeatingMsgs(ss)
		if b {
			log.Print(ss2)
		}
	} else {
		log.Print(ss)
	}
}

func logit(ctx *LogCtx, lvl int, prefix string, o ...interface{}) {
	if _ElogLevel >= lvl {
		ri, _ := GetRuntimeInfo(ctx, 4)
		ss := fmt.Sprint(o...)
		ss2 := fmt.Sprintf("%s %s [%s:%d] ", prefix, ss, ri.FileName, ri.Line)
		core_logit(ctx, ss2)
	}
}

func logitf(ctx *LogCtx, lvl int, prefix string, format string, o ...interface{}) {
	if _ElogLevel >= lvl {
		ri, _ := GetRuntimeInfo(ctx, 4)
		ss := fmt.Sprintf(format, o...)
		ss2 := fmt.Sprintf("%s %s [%s:%d] ", prefix, ss, ri.FileName, ri.Line)
		core_logit(ctx, ss2)
	}
}

func traceLogitf(ctx *LogCtx, format string, o ...interface{}) {
	if _ElogTraceCnt == ELOG_TRACE_ALL || _ElogTraceCnt > 0 {
		ri, _ := GetRuntimeInfo(ctx, 4)
		logIt := false
		for _, it := range _ElogTraceFiles {
			if it == "*" || strings.Index(ri.FileName, it) > -1 {
				logIt = true
				break
			}
		}
		if logIt {
			ss := fmt.Sprintf(format, o...)
			ss2 := fmt.Sprintf("%s/%d %s [%s:%d] ", ELOG_TRACE_PREFIX, _ElogTraceCnt, ss, ri.FileName, ri.Line)
			logIt := true
			if len(_ElogTracePatterns) > 0 {
				logIt = false
				for _, pat := range _ElogTracePatterns {
					if strings.Index(ss2, pat) > -1 {
						logIt = true
						break
					}
				}
			}
			if logIt {
				core_logit(ctx, ss2)
				if _ElogTraceCnt > 0 {
					_ElogTraceCnt--
				}
			}
		}
	}
}

func AppendFileInfo(ctx *LogCtx, err error, depth int) error {
	if _ElogErrorAppendFileLine && err != nil {
		ss := err.Error()
		lastRune := ss[len(ss)-1]
		if lastRune != '/' {
			ri, _ := GetRuntimeInfo(ctx, depth)
			sfi := strings.Replace(fmt.Sprintf("%s:%d", ri.FileName, ri.Line), ".go", "", -1)
			if _ElogObfuscateFileLine {
				sfi = ObfuscateText(sfi)
			}
			if ctx != nil && ctx.Prefix != "" {
				return fmt.Errorf("%s - %v /%s/", ctx.Prefix, err, sfi)
			}
			return fmt.Errorf("%v /%s/", err, sfi)
		}
	}
	return err
}

func logAndRet(ctx *LogCtx, lvl int, prefix string, err error) error {
	if err == nil {
		return nil
	}
	if _ElogLevel >= lvl {
		ri, _ := GetRuntimeInfo(ctx, 4)
		ss := fmt.Sprintf("lar - [%v] %v", reflect.TypeOf((err)), err)
		ss2 := fmt.Sprintf("%s %s [%s:%d] ", prefix, ss, ri.FileName, ri.Line)
		core_logit(ctx, ss2)
	}
	return AppendFileInfo(ctx, err, 5)
}

func Elar(err error) error {
	return logAndRet(nil, ELOG_ERROR, ELOG_ERROR_MSG, err)
}

func ElarCtx(ctx *LogCtx, err error) error {
	return logAndRet(ctx, ELOG_ERROR, ELOG_ERROR_MSG, err)
}

func Elarf(format string, o ...interface{}) error {
	err := fmt.Errorf(format, o...)
	return logAndRet(nil, ELOG_ERROR, ELOG_ERROR_MSG, err)
}

func ElarCtxf(ctx *LogCtx, format string, o ...interface{}) error {
	err := fmt.Errorf(format, o...)
	return logAndRet(ctx, ELOG_ERROR, ELOG_ERROR_MSG, err)
}

func Debug(o ...interface{}) {
	logit(nil, ELOG_DEBUG, ELOG_DEBUG_MSG, o...)
}

func Debugf(format string, o ...interface{}) {
	logitf(nil, ELOG_DEBUG, ELOG_DEBUG_MSG, format, o...)
}

// Nekdy potrebujeme testovat zda je trace zapnuty pred tim, nez
// zavolame Tracef/TraceCtxf kuli performace, protoze nekdy nechceme
// aby se vubec volalo formatovani do textove hlasky (kuli vykonu)
func IsTrace() bool {
	return _ElogTraceCnt == ELOG_TRACE_ALL || _ElogTraceCnt > 0
}
func Tracef(format string, o ...interface{}) {
	traceLogitf(nil, format, o...)
}

func TraceCtxf(ctx *LogCtx, format string, o ...interface{}) {
	traceLogitf(ctx, format, o...)
}

func Info(o ...interface{}) {
	logit(nil, ELOG_INFO, ELOG_INFO_MSG, o...)
}

func Infof(format string, o ...interface{}) {
	logitf(nil, ELOG_INFO, ELOG_INFO_MSG, format, o...)
}

func InfoCtxf(ctx *LogCtx, format string, o ...interface{}) {
	logitf(ctx, ELOG_INFO, ELOG_INFO_MSG, format, o...)
}

func Warn(o ...interface{}) {
	logit(nil, ELOG_WARN, ELOG_WARN_MSG, o...)
}

func Warnf(format string, o ...interface{}) {
	logitf(nil, ELOG_WARN, ELOG_WARN_MSG, format, o...)
}

func Error(o ...interface{}) {
	logit(nil, ELOG_ERROR, ELOG_ERROR_MSG, o...)
}

func Errorf(format string, o ...interface{}) {
	logitf(nil, ELOG_ERROR, ELOG_ERROR_MSG, format, o...)
}

func getLineFromStack(skipFlt string) string {
	stack := debug.Stack()
	reSkip, err := regexp.Compile(skipFlt)
	if err != nil {
		return fmt.Sprintf("getLineFromStack - Invalid RE: %v", skipFlt)
	}
	lnFlt := ".*:[0-9]+ [+][0-9x]+"
	re, err := regexp.Compile(lnFlt)
	if err != nil {
		return fmt.Sprintf("getLineFromStack - Invalid RE: %v", lnFlt)
	}
	aa := strings.Split(string(stack), "\n")
	for _, ln := range aa {
		if re.MatchString(ln) && !reSkip.MatchString(ln) {
			pos := strings.LastIndex(ln, "/")
			if pos == -1 {
				return ln
			} else {
				return ln[pos+1:]
			}
		}
	}
	return "getLineFromStack not found"
}

func Panic(o ...interface{}) {
	logit(nil, ELOG_PANIC, ELOG_PANIC_MSG, o...)
	ss := fmt.Sprint(o...)
	stack := debug.Stack()
	logit(nil, ELOG_PANIC, ELOG_PANIC_MSG, string(stack))
	panic(PanicT(ss))
}

func PanicEx(flt string, o ...interface{}) {
	ln := getLineFromStack(flt)
	logit(nil, ELOG_PANIC, ELOG_PANIC_MSG, fmt.Sprintf("### PANIC LINE: %s", ln))
	logit(nil, ELOG_PANIC, ELOG_PANIC_MSG, o...)
	ss := fmt.Sprint(o...)
	stack := debug.Stack()
	logit(nil, ELOG_PANIC, ELOG_PANIC_MSG, string(stack))
	panic(PanicT(ss))
}

func Panicf(format string, o ...interface{}) {
	logitf(nil, ELOG_PANIC, ELOG_PANIC_MSG, format, o...)
	stack := debug.Stack()
	logit(nil, ELOG_PANIC, ELOG_PANIC_MSG, string(stack))
	ss := fmt.Sprintf(format, o...)
	panic(PanicT(ss))
}

func PanicfEx(flt string, format string, o ...interface{}) {
	ln := getLineFromStack(flt)
	logit(nil, ELOG_PANIC, ELOG_PANIC_MSG, fmt.Sprintf("### PANIC LINE: %s", ln))
	logitf(nil, ELOG_PANIC, ELOG_PANIC_MSG, format, o...)
	stack := debug.Stack()
	logit(nil, ELOG_PANIC, ELOG_PANIC_MSG, string(stack))
	ss := fmt.Sprintf(format, o...)
	panic(PanicT(ss))
}

func Unrechable() {
	Panic("unrechable")
}

func fileSizeRotate(fname string, max_file_size int64) {
	fi, err := _ElogFile.Stat()
	if err != nil {
		Errorf("Cannot obtain file size: %v", err)
	} else {
		if fi.Size() > max_file_size {
			Infof("File size over: %d rotate...", max_file_size)
			//na chvili hod stderr
			_ElogFile.Sync()
			log.SetOutput(os.Stderr)
			//nastav novy soubor
			_ElogFile.Close()

			switch _ElogRotateMode {
			case ELOG_ROTATE_MODE_TIME:
				timestamp := time.Now().Format("20060102-150405")
				err = os.Rename(fname, fmt.Sprintf("%s.%s", fname, timestamp))
				if err != nil {
					log.Panicf("Cannot Rename file: %s error: %v", fname, err)
				}
			case ELOG_ROTATE_MODE_2FILES:
				err = os.Rename(fname, fmt.Sprintf("%s.0", fname))
				if err != nil {
					log.Panicf("Cannot Rename file: %s error: %v", fname, err)
				}
			default:
				Panicf("Invalid rotate mode: %v", _ElogRotateMode)
			}

			_ElogFile, err = os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
			if err != nil {
				log.Panicf("Cannot OpenFile: %s", fname)
			}
			log.SetOutput(_ElogFile)
		}
	}
}

func fileSizeWatcher(fname string, maxFileSize int64) {
	for {
		rnd := rand.Intn(60)
		time.Sleep(time.Duration(rnd+60) * time.Second)

		fileSizeRotate(fname, maxFileSize)
	}
}

// max_file_size = 0 ignore
func SetLogFile(fname string, maxFileSize int64) {
	var err error
	_ElogFile, err = os.OpenFile(fname, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		log.Panicf("Cannot OpenFile: %s", fname)
	}
	log.SetOutput(_ElogFile)
	fmt.Printf("Log file set to: %s\n", fname)
	if maxFileSize > 0 {
		fileSizeRotate(fname, maxFileSize)
		go fileSizeWatcher(fname, maxFileSize)
	}
}

func RecoverPanic(fn func()) (recovered interface{}) {
	defer func() {
		recovered = recover()
	}()
	fn()
	return
}

func ObfuscateText(text string) string {
	seed := rune(rand.Intn(0xFF))
	plus := rune(seed * 8)
	obfuscated := make([]byte, len(text))

	inc := seed
	for i, r := range text {
		inc += plus
		obfuscated[i] = byte(int(r) ^ int(inc))
	}

	obfuscatedHex := hex.EncodeToString(obfuscated)
	seedHex := fmt.Sprintf("%02x", int(seed))

	obfuscatedWithSeed := fmt.Sprintf("%s%s", seedHex, obfuscatedHex)

	return obfuscatedWithSeed
}

func DeobfuscateText(obfuscated string) (string, error) {
	seedHex := obfuscated[:2]
	obfuscatedHex := obfuscated[2:]

	seedInt, err := hex.DecodeString(seedHex)
	if err != nil {
		return "", err
	}
	seed := rune(seedInt[0])

	obfuscatedBytes, err := hex.DecodeString(obfuscatedHex)
	if err != nil {
		return "", err
	}

	deobfuscated := make([]byte, len(obfuscatedBytes))
	inc := seed
	plus := rune(seed * 8)
	for i, b := range obfuscatedBytes {
		inc += plus
		deobfuscated[i] = b ^ byte(inc)
	}

	return string(deobfuscated), nil
}

func Pp(o ...interface{}) {
	fmt.Print(">> ")
	ri, _ := GetRuntimeInfo(nil, 3)
	pp.Print(o)
	fmt.Printf("  [%s:%d] <<\n", ri.FileName, ri.Line)
}

func dbglnLogit(ctx *LogCtx, skip int, o ...interface{}) {
	if _ElogLevel >= ELOG_DEBUG {
		ri, _ := GetRuntimeInfo(ctx, skip)
		ss := fmt.Sprint(o...)
		prefix := ELOG_DBG_FUNCTION
		ss2 := fmt.Sprintf("%s %s [%s:%d] ", prefix, ss, ri.FileName, ri.Line)
		core_logit(ctx, ss2)
	}
}

func DbglnBase(skip int, o ...interface{}) {

	if _ElogTotalOFF {
		return
	}
	if !_ElogDbgFunctionsToFile {
		var rst string = "\033[0m"

		cols := []string{
			"\033[93m", // yellow
			"\033[96m", // cyan
			"\033[92m", // green
			"\033[95m", // purple
			"\033[97m", // white
			"\033[91m", // red
			"\033[94m", // blue
		}

		aa := []string{}
		for i, v := range o {
			c := cols[i%len(cols)]
			aa = append(aa, fmt.Sprintf("%s%+v", c, v))
		}

		ss := strings.Join(aa, " ")
		ri, _ := GetRuntimeInfo(nil, skip)
		ss2 := fmt.Sprintf(" %s%s%s [%s:%d]", rst, ss, rst, ri.FileName, ri.Line)
		fmt.Println(ss2)
	} else {
		dbglnLogit(nil, 5, o...)
	}
}

func Dbgln(o ...interface{}) {

	DbglnBase(4, o...)
}

func DbglnIf(cond bool, o ...interface{}) {
	if cond {
		DbglnBase(4, o...)
	}
}

/*
func Dbgln2(o ...interface{}) {

	aa := []string{}
	for _, v := range o {
		aa = append(aa, fmt.Sprintf("%v", v))
	}

	ss := strings.Join(aa, " ")
	ri, _ := GetRuntimeInfo(nil, 3)
	ss2 := fmt.Sprintf(" %s [%s:%d]", ss, ri.FileName, ri.Line)
	fmt.Println(ss2)
}
*/

func Dbgf(format string, o ...interface{}) {
	ss := fmt.Sprintf(format, o...)
	args := []interface{}{}
	for _, s := range strings.Split(ss, "__") {
		args = append(args, s)
	}
	DbglnBase(4, args...)
}
