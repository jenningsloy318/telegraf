package wavefront

import (
	"bufio"
	"bytes"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/influxdata/telegraf"
	"github.com/influxdata/telegraf/metric"
	"github.com/influxdata/telegraf/plugins/parsers"
)

const MaxBufferSize = 2

type Point struct {
	Name      string
	Value     string
	Timestamp int64
	Source    string
	Tags      map[string]string
}

type Parser struct {
	parsers     *sync.Pool
	DefaultTags map[string]string `toml:"-"`
	Log         telegraf.Logger   `toml:"-"`
}

// PointParser is a thread-unsafe parser and must be kept in a pool.
type PointParser struct {
	s   *PointScanner
	buf struct {
		tok []Token  // last read n tokens
		lit []string // last read n literals
		n   int      // unscanned buffer size (max=2)
	}
	scanBuf  bytes.Buffer // buffer reused for scanning tokens
	writeBuf bytes.Buffer // buffer reused for parsing elements
	Elements []elementParser
	parent   *Parser
}

// NewWavefrontElements returns a slice of elementParser's for the Graphite format
func NewWavefrontElements() []elementParser {
	var elements []elementParser
	wsParser := whiteSpaceParser{}
	wsParserNextOpt := whiteSpaceParser{nextOptional: true}
	repeatParser := loopedParser{wrappedParser: &tagParser{}, wsParser: &wsParser}
	elements = append(elements, &nameParser{}, &wsParser, &valueParser{}, &wsParserNextOpt,
		&timestampParser{optional: true}, &wsParserNextOpt, &repeatParser)
	return elements
}

func (p *Parser) Init() error {
	p.parsers = &sync.Pool{
		New: func() interface{} {
			elements := NewWavefrontElements()
			return &PointParser{Elements: elements, parent: p}
		},
	}
	return nil
}

func (p *Parser) ParseLine(line string) (telegraf.Metric, error) {
	buf := []byte(line)

	metrics, err := p.Parse(buf)
	if err != nil {
		return nil, err
	}

	if len(metrics) > 0 {
		return metrics[0], nil
	}

	return nil, nil
}

func (p *Parser) Parse(buf []byte) ([]telegraf.Metric, error) {
	pp := p.parsers.Get().(*PointParser)
	defer p.parsers.Put(pp)
	return pp.Parse(buf)
}

func (p *PointParser) Parse(buf []byte) ([]telegraf.Metric, error) {
	// parse even if the buffer begins with a newline
	buf = bytes.TrimPrefix(buf, []byte("\n"))
	// add newline to end if not exists:
	if len(buf) > 0 && !bytes.HasSuffix(buf, []byte("\n")) {
		buf = append(buf, []byte("\n")...)
	}

	points := make([]Point, 0)

	buffer := bytes.NewBuffer(buf)
	reader := bufio.NewReader(buffer)
	for {
		// Read up to the next newline.
		buf, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}

		p.reset(buf)
		point := Point{}
		for _, element := range p.Elements {
			err := element.parse(p, &point)
			if err != nil {
				return nil, err
			}
		}

		points = append(points, point)
	}

	metrics, err := p.convertPointToTelegrafMetric(points)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func (p *Parser) SetDefaultTags(tags map[string]string) {
	p.DefaultTags = tags
}

func (p *PointParser) convertPointToTelegrafMetric(points []Point) ([]telegraf.Metric, error) {
	metrics := make([]telegraf.Metric, 0)

	for _, point := range points {
		tags := make(map[string]string)
		for k, v := range point.Tags {
			tags[k] = v
		}
		// apply default tags after parsed tags
		for k, v := range p.parent.DefaultTags {
			tags[k] = v
		}

		// single field for value
		fields := make(map[string]interface{})
		v, err := strconv.ParseFloat(point.Value, 64)
		if err != nil {
			return nil, err
		}
		fields["value"] = v

		m := metric.New(point.Name, tags, fields, time.Unix(point.Timestamp, 0))

		metrics = append(metrics, m)
	}

	return metrics, nil
}

// scan returns the next token from the underlying scanner.
// If a token has been unscanned then read that from the internal buffer instead.
func (p *PointParser) scan() (Token, string) {
	// If we have a token on the buffer, then return it.
	if p.buf.n != 0 {
		idx := p.buf.n % MaxBufferSize
		tok, lit := p.buf.tok[idx], p.buf.lit[idx]
		p.buf.n--
		return tok, lit
	}

	// Otherwise read the next token from the scanner.
	tok, lit := p.s.Scan()

	// Save it to the buffer in case we unscan later.
	p.buffer(tok, lit)

	return tok, lit
}

func (p *PointParser) buffer(tok Token, lit string) {
	// create the buffer if it is empty
	if len(p.buf.tok) == 0 {
		p.buf.tok = make([]Token, MaxBufferSize)
		p.buf.lit = make([]string, MaxBufferSize)
	}

	// for now assume a simple circular buffer of length two
	p.buf.tok[0], p.buf.lit[0] = p.buf.tok[1], p.buf.lit[1]
	p.buf.tok[1], p.buf.lit[1] = tok, lit
}

// unscan pushes the previously read token back onto the buffer.
func (p *PointParser) unscan() {
	p.unscanTokens(1)
}

func (p *PointParser) unscanTokens(n int) {
	if n > MaxBufferSize {
		// just log for now
		p.parent.Log.Infof("Cannot unscan more than %d tokens", MaxBufferSize)
	}
	p.buf.n += n
}

func (p *PointParser) reset(buf []byte) {
	// reset the scan buffer and write new byte
	p.scanBuf.Reset()
	p.scanBuf.Write(buf)

	if p.s == nil {
		p.s = NewScanner(&p.scanBuf)
	} else {
		// reset p.s.r passing in the buffer as the reader
		p.s.r.Reset(&p.scanBuf)
	}
	p.buf.n = 0
}

func init() {
	parsers.Add("wavefront",
		func(string) telegraf.Parser {
			return &Parser{}
		})
}
