package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/alecthomas/kong"
	"github.com/hlfshell/scaffold"
)

const defaultTimeout = 5 * time.Minute

type App struct {
	name        string
	description string
	service     scaffold.Service
	envFile     string
	timeout     time.Duration
	out         io.Writer
	err         io.Writer
	commands    []Command
}

type Option func(*App)

type Command struct {
	Name string
	Help string
	Run  func(context.Context, scaffold.Service, []string) error
}

type CommandProvider interface {
	CLICommands() []Command
}

type outputContextKey struct{}

func Fprintf(ctx context.Context, format string, args ...any) {
	writer, ok := ctx.Value(outputContextKey{}).(io.Writer)
	if !ok || writer == nil {
		writer = os.Stdout
	}

	fmt.Fprintf(writer, format, args...)
}

func Fprintln(ctx context.Context, args ...any) {
	writer, ok := ctx.Value(outputContextKey{}).(io.Writer)
	if !ok || writer == nil {
		writer = os.Stdout
	}

	fmt.Fprintln(writer, args...)
}

/*
New creates a command-line controller for a scaffold service or stack.
Stacks expose the richest command set because they can report summaries,
environment variables, endpoints, and Docker resources.
*/
func New(name string, service scaffold.Service, options ...Option) *App {
	app := &App{
		name:        name,
		description: fmt.Sprintf("manage the %s scaffold service", name),
		service:     service,
		envFile:     ".env.scaffold",
		timeout:     defaultTimeout,
		out:         os.Stdout,
		err:         os.Stderr,
	}

	for _, option := range options {
		option(app)
	}

	return app
}

func WithDescription(description string) Option {
	return func(app *App) {
		app.description = description
	}
}

func WithEnvFile(path string) Option {
	return func(app *App) {
		app.envFile = path
	}
}

func WithTimeout(timeout time.Duration) Option {
	return func(app *App) {
		app.timeout = timeout
	}
}

func WithWriters(out io.Writer, err io.Writer) Option {
	return func(app *App) {
		app.out = out
		app.err = err
	}
}

func WithCommand(command Command) Option {
	return func(app *App) {
		app.commands = append(app.commands, command)
	}
}

func WithCommands(commands ...Command) Option {
	return func(app *App) {
		app.commands = append(app.commands, commands...)
	}
}

func (app *App) Run(args []string) int {
	if app.service == nil {
		fmt.Fprintln(app.err, "scaffold cli requires a service")
		return 1
	}
	app.commands = normalizeCommands(app.service, app.commands)

	if len(args) == 0 || isTopLevelHelp(args) {
		app.printHelp()
		return 0
	}

	root := &commands{
		app: app,
		Once: onceCommand{
			EnvFile: app.envFile,
		},
		Up: upCommand{
			EnvFile: app.envFile,
		},
		Run: runCommand{
			EnvFile: app.envFile,
		},
		Env: envCommand{
			Format: "dotenv",
		},
	}

	parser, err := kong.New(
		root,
		kong.Name(app.name),
		kong.Description(app.description),
		kong.Writers(app.out, app.err),
		kong.Exit(func(int) {}),
	)
	if err != nil {
		fmt.Fprintln(app.err, err)
		return 1
	}

	ctx, err := parser.Parse(args)
	if err != nil {
		if hasHelpFlag(args) {
			return 0
		}
		var parseErr *kong.ParseError
		if errors.As(err, &parseErr) && parseErr.Context != nil {
			app.printHelp()
			fmt.Fprintln(app.out)
		}
		fmt.Fprintln(app.err, err)
		return 2
	}

	err = ctx.Run(app)
	if err != nil {
		fmt.Fprintln(app.err, err)
		return 1
	}

	return 0
}

func (app *App) MustRun(args []string) {
	os.Exit(app.Run(args))
}

type commands struct {
	app *App

	Up        upCommand        `cmd:"" help:"Start the service and keep it running until interrupted."`
	Down      downCommand      `cmd:"" help:"Stop and remove resources for this service or stack."`
	Once      onceCommand      `cmd:"" help:"Start the service, print connection details, then clean up."`
	Run       runCommand       `cmd:"" help:"Start the service, run a registered command, then clean up."`
	Status    statusCommand    `cmd:"" help:"Show services, sub-services, and matching Docker resources."`
	Summary   summaryCommand   `cmd:"" help:"Print a stack or service summary."`
	Env       envCommand       `cmd:"" help:"Print environment variables exposed by the service."`
	Endpoints endpointsCommand `cmd:"" help:"Print endpoints exposed by the service."`
	Resources resourcesCommand `cmd:"" help:"Print Docker resources discovered for the stack."`
	Live      liveCommand      `cmd:"" help:"Open an interactive Bubble Tea controller."`
}

type upCommand struct {
	EnvFile string `help:"Write environment variables to this dotenv file."`
	NoEnv   bool   `help:"Do not write an env file."`
}

func (cmd upCommand) Run(app *App) error {
	cmd.EnvFile = app.envFilePath(cmd.EnvFile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithTimeout(ctx, app.timeout)
	defer cancel()

	if err := create(ctx, app.service); err != nil {
		return err
	}

	if err := app.printInspect(ctx, cmd.EnvFile, cmd.NoEnv); err != nil {
		_ = cleanup(context.Background(), app.service)
		return err
	}

	fmt.Fprintln(app.out)
	fmt.Fprintln(app.out, "Running. Press Ctrl+C to clean up.")
	<-ctx.Done()

	return cleanup(context.Background(), app.service)
}

type downCommand struct{}

func (downCommand) Run(app *App) error {
	ctx, cancel := context.WithTimeout(context.Background(), app.timeout)
	defer cancel()

	if err := down(ctx, app.service); err != nil {
		return err
	}

	fmt.Fprintln(app.out, "Stopped and removed matching resources.")
	return nil
}

type onceCommand struct {
	EnvFile string `help:"Write environment variables to this dotenv file."`
	NoEnv   bool   `help:"Do not write an env file."`
}

func (cmd onceCommand) Run(app *App) error {
	cmd.EnvFile = app.envFilePath(cmd.EnvFile)

	ctx, cancel := context.WithTimeout(context.Background(), app.timeout)
	defer cancel()

	if err := create(ctx, app.service); err != nil {
		return err
	}

	inspectErr := app.printInspect(ctx, cmd.EnvFile, cmd.NoEnv)
	cleanupErr := cleanup(context.Background(), app.service)
	if inspectErr != nil && cleanupErr != nil {
		return fmt.Errorf("%w; cleanup failed: %v", inspectErr, cleanupErr)
	}
	if inspectErr != nil {
		return inspectErr
	}

	return cleanupErr
}

type runCommand struct {
	EnvFile string   `help:"Write environment variables to this dotenv file."`
	NoEnv   bool     `help:"Do not write an env file."`
	Command []string `arg:"" optional:"" passthrough:"" help:"Registered command name and optional arguments."`
}

func (cmd runCommand) Run(app *App) error {
	cmd.EnvFile = app.envFilePath(cmd.EnvFile)
	cmd.Command = stripPassthroughSeparator(cmd.Command)

	if len(cmd.Command) == 0 {
		app.printCommands()
		return nil
	}
	if len(app.commands) == 0 {
		return fmt.Errorf("no commands are registered")
	}

	registered, ok := app.command(cmd.Command[0])
	if !ok {
		app.printCommands()
		return fmt.Errorf("unknown command %q", cmd.Command[0])
	}

	ctx, cancel := context.WithTimeout(context.Background(), app.timeout)
	defer cancel()

	if err := create(ctx, app.service); err != nil {
		return err
	}

	runErr := app.printInspect(ctx, cmd.EnvFile, cmd.NoEnv)
	if runErr == nil {
		runErr = runRegisteredCommand(ctx, app, registered, cmd.Command[1:])
	}

	cleanupErr := cleanup(context.Background(), app.service)
	if runErr != nil && cleanupErr != nil {
		return fmt.Errorf("%w; cleanup failed: %v", runErr, cleanupErr)
	}
	if runErr != nil {
		return runErr
	}

	return cleanupErr
}

type statusCommand struct{}

func (statusCommand) Run(app *App) error {
	ctx, cancel := context.WithTimeout(context.Background(), app.timeout)
	defer cancel()

	report, err := status(ctx, app.service)
	if err != nil {
		return err
	}

	printStatus(app.out, report)
	return nil
}

type summaryCommand struct{}

func (summaryCommand) Run(app *App) error {
	fmt.Fprintln(app.out, summary(app.service))
	return nil
}

type envCommand struct {
	Format string `help:"Output format: dotenv or table." enum:"dotenv,table" default:"dotenv"`
}

func (cmd envCommand) Run(app *App) error {
	env := env(app.service)
	if len(env) == 0 {
		fmt.Fprintln(app.out, "No environment variables exposed.")
		return nil
	}

	if cmd.Format == "table" {
		printMap(app.out, "Environment", env)
		return nil
	}

	for _, key := range sortedKeys(env) {
		fmt.Fprintf(app.out, "%s=%s\n", key, quoteEnv(env[key]))
	}
	return nil
}

type endpointsCommand struct{}

func (endpointsCommand) Run(app *App) error {
	endpoints := endpoints(app.service)
	if len(endpoints) == 0 {
		fmt.Fprintln(app.out, "No endpoints exposed.")
		return nil
	}

	printMap(app.out, "Endpoints", endpoints)
	return nil
}

type resourcesCommand struct{}

func (resourcesCommand) Run(app *App) error {
	ctx, cancel := context.WithTimeout(context.Background(), app.timeout)
	defer cancel()

	resources, ok, err := resources(ctx, app.service)
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(app.out, "This service does not expose Docker resource discovery.")
		return nil
	}

	printResources(app.out, resources)
	return nil
}

func (app *App) printInspect(ctx context.Context, envFile string, noEnv bool) error {
	fmt.Fprintln(app.out, summary(app.service))

	if !noEnv {
		if err := writeEnvFile(envFile, env(app.service)); err != nil {
			return err
		}
		fmt.Fprintln(app.out)
		fmt.Fprintf(app.out, "Wrote env file: %s\n", envFile)
	}

	printMap(app.out, "Environment", env(app.service))
	printMap(app.out, "Endpoints", endpoints(app.service))

	resources, ok, err := resources(ctx, app.service)
	if err != nil {
		return err
	}
	if ok {
		fmt.Fprintln(app.out)
		printResources(app.out, resources)
	}

	return nil
}

func (app *App) envFilePath(path string) string {
	if path != "" {
		return path
	}

	return app.envFile
}

func (app *App) command(name string) (Command, bool) {
	for _, command := range app.commands {
		if command.Name == name {
			return command, true
		}
	}

	return Command{}, false
}

func (app *App) printCommands() {
	app.printCommandsTo(app.out)
}

func (app *App) printCommandsTo(out io.Writer) {
	fmt.Fprintln(out, "Commands:")
	if len(app.commands) == 0 {
		fmt.Fprintln(out, "  none")
		return
	}

	for _, command := range app.commands {
		fmt.Fprintf(out, "  %s", command.Name)
		if command.Help != "" {
			fmt.Fprintf(out, "  %s", command.Help)
		}
		fmt.Fprintln(out)
	}
}

func (app *App) printHelp() {
	fmt.Fprintf(app.out, "Usage: %s <command>\n\n", app.name)
	if app.description != "" {
		fmt.Fprintln(app.out, app.description)
		fmt.Fprintln(app.out)
	}

	fmt.Fprintln(app.out, "Flags:")
	fmt.Fprintln(app.out, "  -h, --help    Show context-sensitive help.")
	fmt.Fprintln(app.out)
	fmt.Fprintln(app.out, "Commands:")
	app.printHelpCommand("up [flags]", "Start the service and keep it running until interrupted.")
	app.printHelpCommand("down", "Stop and remove resources for this service or stack.")
	if len(app.commands) > 0 {
		app.printHelpCommand("run [<command> ...] [flags]", "Start the service, run a registered command, then clean up. Without a command, list registered commands.")
	}
	app.printHelpCommand("status", "Show services, sub-services, and matching Docker resource state.")
	app.printHelpCommand("summary", "Print a stack or service summary.")
	app.printHelpCommand("env [flags]", "Print environment variables exposed by the service.")
	if hasEndpoints(app.service) {
		app.printHelpCommand("endpoints", "Print endpoints exposed by the service.")
	}
	app.printHelpCommand("resources", "Print Docker resources discovered for the stack.")
	app.printHelpCommand("live", "Open an interactive Bubble Tea controller.")
	fmt.Fprintf(app.out, "\nRun \"%s <command> --help\" for more information on a command.\n", app.name)
}

func (app *App) printHelpCommand(name string, description string) {
	fmt.Fprintf(app.out, "  %s\n", name)
	fmt.Fprintf(app.out, "    %s\n\n", description)
}

func hasHelpFlag(args []string) bool {
	for _, arg := range args {
		if arg == "-h" || arg == "--help" || arg == "help" {
			return true
		}
	}

	return false
}

func isTopLevelHelp(args []string) bool {
	return len(args) == 1 && hasHelpFlag(args)
}

func stripPassthroughSeparator(args []string) []string {
	if len(args) > 0 && args[0] == "--" {
		return args[1:]
	}

	return args
}

func create(ctx context.Context, service scaffold.Service) error {
	return service.Create(ctx)
}

func cleanup(ctx context.Context, service scaffold.Service) error {
	return service.Cleanup(ctx)
}

func down(ctx context.Context, service scaffold.Service) error {
	downer, ok := service.(interface {
		Down(context.Context) error
	})
	if ok {
		return downer.Down(ctx)
	}

	return cleanup(ctx, service)
}

func summary(service scaffold.Service) string {
	summarizer, ok := service.(interface{ Summary() string })
	if ok {
		return summarizer.Summary()
	}

	return fmt.Sprintf("Service %s", service.Name())
}

func env(service scaffold.Service) map[string]string {
	provider, ok := service.(scaffold.EnvProvider)
	if !ok {
		return map[string]string{}
	}

	return provider.Env()
}

func endpoints(service scaffold.Service) map[string]string {
	provider, ok := service.(scaffold.EndpointProvider)
	if !ok {
		return map[string]string{}
	}

	return provider.Endpoints()
}

func hasEndpoints(service scaffold.Service) bool {
	return len(endpoints(service)) > 0
}

func resources(ctx context.Context, service scaffold.Service) (scaffold.ResourceStatus, bool, error) {
	provider, ok := service.(interface {
		Resources(context.Context) (scaffold.ResourceStatus, error)
	})
	if ok {
		resources, err := provider.Resources(ctx)
		return resources, true, err
	}

	return scaffold.ResourceStatus{}, false, nil
}

func writeEnvFile(path string, values map[string]string) error {
	lines := []string{}
	for _, key := range sortedKeys(values) {
		lines = append(lines, fmt.Sprintf("%s=%s", key, quoteEnv(values[key])))
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0644)
}

func printMap(out io.Writer, title string, values map[string]string) {
	fmt.Fprintln(out)
	fmt.Fprintln(out, title+":")
	if len(values) == 0 {
		fmt.Fprintln(out, "  none")
		return
	}

	for _, key := range sortedKeys(values) {
		fmt.Fprintf(out, "  %s=%s\n", key, values[key])
	}
}

func printResources(out io.Writer, resources scaffold.ResourceStatus) {
	fmt.Fprintln(out, "Resources:")

	if len(resources.Containers) == 0 && len(resources.Networks) == 0 && len(resources.Volumes) == 0 {
		fmt.Fprintln(out, "  none")
		return
	}

	for _, container := range resources.Containers {
		fmt.Fprintf(out, "  container  %s  %s  %s\n", strings.TrimPrefix(container.Name, "/"), container.State, container.Status)
	}
	for _, network := range resources.Networks {
		fmt.Fprintf(out, "  network    %s  %s\n", network.Name, network.Driver)
	}
	for _, volume := range resources.Volumes {
		fmt.Fprintf(out, "  volume     %s  %s\n", volume.Name, volume.Driver)
	}
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	return keys
}

func quoteEnv(value string) string {
	if value == "" {
		return `""`
	}
	if strings.ContainsAny(value, " \t\n\"'") {
		return fmt.Sprintf("%q", value)
	}

	return value
}

func normalizeCommands(service scaffold.Service, commands []Command) []Command {
	provider, ok := service.(CommandProvider)
	if ok {
		commands = append(commands, provider.CLICommands()...)
	}

	seen := map[string]struct{}{}
	normalized := []Command{}
	for _, command := range commands {
		if command.Name == "" || command.Run == nil {
			continue
		}
		if _, ok := seen[command.Name]; ok {
			continue
		}

		seen[command.Name] = struct{}{}
		normalized = append(normalized, command)
	}

	return normalized
}

func runRegisteredCommand(ctx context.Context, app *App, command Command, args []string) error {
	return runRegisteredCommandWithOutput(ctx, app, command, args, app.out)
}

func runRegisteredCommandWithOutput(ctx context.Context, app *App, command Command, args []string, out io.Writer) error {
	ctx = context.WithValue(ctx, outputContextKey{}, out)
	return command.Run(ctx, app.service, args)
}
