package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
	flag "github.com/spf13/pflag"

	"github.com/dtan4/s3url/aws"
	"github.com/dtan4/s3url/config"
)

const (
	exitCodeOK int = iota
	exitCodeError
)

const usage = `Usage of %s:
   %s https://s3-region.amazonaws.com/BUCKET/KEY [-d DURATION]
   %s s3://BUCKET/KEY [-d DURATION]
   %s -b BUCKET -k KEY [-d DURATION]

Options:
`

// CLI represent CLI implementation
type CLI struct {
	stdout   io.Writer
	stderr   io.Writer
	version  string
	revision string
}

// New returns new CLI object
func New(stdout, stderr io.Writer, version, revision string) *CLI {
	return &CLI{
		stdout:   stdout,
		stderr:   stderr,
		version:  version,
		revision: revision,
	}
}

// Run executes s3url command process
func (cli *CLI) Run(args []string) int {
	f := flag.NewFlagSet("s3url", flag.ExitOnError)

	f.Usage = func() {
		fmt.Fprintf(cli.stderr, usage, args[0], args[0], args[0], args[0])
		f.PrintDefaults()
	}

	c := config.Config{}

	f.StringVarP(&c.Bucket, "bucket", "b", "", "Bucket name")
	f.BoolVar(&c.Debug, "debug", false, "Enable debug output")
	f.Int64VarP(&c.Duration, "duration", "d", config.DefaultDuration, "Valid duration in minutes")
	f.StringVarP(&c.Key, "key", "k", "", "Object key")
	f.StringVar(&c.Profile, "profile", "", "AWS profile name")
	f.StringVar(&c.Upload, "upload", "", "File to upload")
	f.BoolVarP(&c.Version, "version", "v", false, "Print version")

	f.Parse(args[1:])

	if c.Version {
		cli.printVersion()
		return exitCodeOK
	}

	var s3URL string

	for 0 < f.NArg() {
		s3URL = f.Args()[0]
		f.Parse(f.Args()[1:])
	}

	if s3URL == "" && (c.Bucket == "" || c.Key == "") {
		f.Usage()
		return exitCodeError
	}

	if s3URL != "" {
		if err := c.ParseS3URL(s3URL); err != nil {
			cli.printError(err, c.Debug)
			return exitCodeError
		}
	}

	if err := c.Validate(); err != nil {
		cli.printError(err, c.Debug)
		return exitCodeError
	}

	if err := aws.Initialize(c.Profile); err != nil {
		cli.printError(err, c.Debug)
		return exitCodeError
	}

	if c.Upload != "" {
		if err := cli.uploadFile(c.Bucket, c.Key, c.Upload); err != nil {
			cli.printError(err, c.Debug)
			return exitCodeError
		}

		fmt.Fprintln(cli.stderr, "uploaded: "+c.Upload)
	}

	signedURL, err := aws.S3.GetPresignedURL(c.Bucket, c.Key, c.Duration)
	if err != nil {
		cli.printError(err, c.Debug)
		return exitCodeError
	}

	fmt.Fprintln(cli.stdout, signedURL)

	return exitCodeOK
}

func (cli *CLI) uploadFile(bucket, key, filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return errors.Wrapf(err, "cannot open %q", filename)
	}
	defer f.Close()

	if err := aws.S3.UploadToS3(bucket, key, f); err != nil {
		return errors.Wrapf(err, "cannot uplaod %q to S3", filename)
	}

	return nil
}

func (cli *CLI) printError(err error, debug bool) {
	if debug {
		fmt.Fprintf(cli.stderr, "%+v\n", err)
	} else {
		fmt.Fprintln(cli.stderr, err)
	}
}

func (cli *CLI) printVersion() {
	fmt.Fprintln(cli.stdout, "s3url version "+cli.version+", build "+cli.revision)
}
