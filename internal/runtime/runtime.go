package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	hostappconfig "github.com/0xa1bed0/mkenv/internal/apps/mkenv/config"
	"github.com/0xa1bed0/mkenv/internal/logs"
	"github.com/0xa1bed0/mkenv/internal/state"
)

type RuntimeType string

const (
	RuntimeTypeHost  RuntimeType = "host"
	RuntimeTypeAgent RuntimeType = "agent"
)

type Runtime struct {
	runID string
	t     RuntimeType

	ctx        context.Context    // global context
	cancelFunc context.CancelFunc // cancelFunc of global context

	mu sync.Mutex

	project *Project

	container *ContainerConfig

	wg              sync.WaitGroup
	shutdownTimeout time.Duration

	term *TerminalGuard

	firstFailErr error
}

func (rt *Runtime) Type() RuntimeType {
	return rt.t
}

func (rt *Runtime) CancelCtx() {
	rt.cancelFunc()
}

func (rt *Runtime) Ctx() context.Context {
	return rt.ctx
}

func (rt *Runtime) GOOS() string {
	return runtime.GOOS
}

func (rt *Runtime) Project() *Project {
	return rt.project
}

func (rt *Runtime) RunID() string {
	return rt.runID
}

func (rt *Runtime) Container() *ContainerConfig {
	return rt.container
}

func (rt *Runtime) Term() *TerminalGuard {
	return rt.term
}

type runtimeKey struct{}

func NewHostRuntime() *Runtime {
	baseCtx, cancel := context.WithCancel(context.Background())
	runID := strconv.FormatInt(time.Now().Unix(), 10)
	rt := &Runtime{
		runID:      runID,
		t:          RuntimeTypeHost,
		cancelFunc: cancel,
		// TODO: try to resolve container for this project (by labels). ask user if many found.
		container:       NewContainerConfig(),
		term:            NewTerminalGuard(),
		shutdownTimeout: time.Duration(5 * time.Second),
	}
	// yes, for this particula case we use context as DI which is very bad practice.
	// BUT we use it for ONLY ONE Runtime pointer.
	// We will make sure we load context from context once at the root of each individual commands
	// this will significantly reduce the boilerplate which is greater win then code readability loss.
	// We have to know only this particular quirck of the system, and only at the cmd level and thats it.
	// we would never get runtime from context everywhere else except the cmd handler function.
	// TODO: maybe we should make Runtime as a singletone so we can access it everywhere - we don't like singletones tho
	// TODO: maybe we should restrict FromContext to be called only once, and after call it - we should throw panic - idk
	ctx := context.WithValue(baseCtx, runtimeKey{}, rt)
	rt.ctx = ctx
	return rt
}

func NewAgentRuntime() *Runtime {
	baseCtx, cancel := context.WithCancel(context.Background())
	rt := &Runtime{
		t:               RuntimeTypeAgent,
		cancelFunc:      cancel,
		shutdownTimeout: time.Duration(5 * time.Second),
	}
	// yes, for this particula case we use context as DI which is very bad practice.
	// BUT we use it for ONLY ONE Runtime pointer.
	// We will make sure we load context from context once at the root of each individual commands
	// this will significantly reduce the boilerplate which is greater win then code readability loss.
	// We have to know only this particular quirck of the system, and only at the cmd level and thats it.
	// we would never get runtime from context everywhere else except the cmd handler function.
	// TODO: maybe we should make Runtime as a singletone so we can access it everywhere - we don't like singletones tho
	// TODO: maybe we should restrict FromContext to be called only once, and after call it - we should throw panic - idk
	ctx := context.WithValue(baseCtx, runtimeKey{}, rt)
	rt.ctx = ctx
	return rt
}

func FromContext(ctx context.Context) *Runtime {
	v := ctx.Value(runtimeKey{})
	if v == nil {
		return nil
	}
	rt, _ := v.(*Runtime)
	return rt
}

func FromContextOrPanic(ctx context.Context) *Runtime {
	rt := FromContext(ctx)
	if rt == nil {
		panic(errors.New("runtime not found in this context"))
	}
	return rt
}

func (rt *Runtime) ResolveProject(ctx context.Context, path string, kvStore *state.KVStore) (*Project, error) {
	if rt.project != nil {
		return rt.project, nil
	}

	var project *Project
	var err error

	project, err = resolveProject(ctx, path, newProjectStateDB(kvStore))
	if err != nil {
		return nil, err
	}

	rt.project = project

	if rt.t == RuntimeTypeHost {
		logs.SetFullLogPath(hostappconfig.RunLogPath(project.Name(), rt.RunID()))
	}

	return rt.project, nil
}

// Go runs fn in a new goroutine, with panic recovery.
//
// Contract:
//   - fn gets rt.Ctx().
//   - If fn returns a non-nil error, Runtime records the first error
//     and cancels the context.
//   - If fn panics, the panic is recovered, wrapped into an error, recorded,
//     and the context is cancelled.
//   - Runtime.Wait() will wait for all such goroutines and return the first error.
func (rt *Runtime) Go(fn func()) {
	rt.wg.Go(func() {
		defer func() {
			// recover panic
			if r := recover(); r != nil {
				err := fmt.Errorf("panic: %v\n%s", r, debug.Stack())
				rt.mu.Lock()
				if rt.firstFailErr == nil {
					rt.firstFailErr = err
					// cancel everyone on first failure
					rt.cancelFunc()
				}
				rt.mu.Unlock()
			}
		}()

		fn()
	})
}

func (rt *Runtime) Wait() error {
	rt.wg.Wait()

	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.firstFailErr
}

func (rt *Runtime) OnShutdown(fn func(ctx context.Context)) {
	rt.Go(func() {
		// wait until runtime context is cancelled
		<-rt.ctx.Done()

		cleanupCtx, cancel := context.WithTimeout(context.Background(), rt.shutdownTimeout)
		defer cancel()

		fn(cleanupCtx)
	})
}

// Finalize handles both panic and normal exit.
// Call it in a defer at the top of main.
func (rt *Runtime) Finalize(appName, helpHint string, execErr *error) {
	// detect panic
	if r := recover(); r != nil {
		// panic path
		if rt.term != nil {
			rt.term.Restore()
		}

		fmt.Fprintf(os.Stderr, "%s panic: %v\n", appName, r)
		fmt.Fprintf(os.Stderr, "%s\n", debug.Stack())
		fmt.Fprintln(os.Stderr, "")
		if helpHint != "" {
			fmt.Fprintln(os.Stderr, helpHint)
		}

		// cancel & wait so OnShutdown hooks run
		rt.CancelCtx()
		_ = rt.Wait()

		logs.Close()
		os.Exit(1)
	}

	// normal (non-panic) path – use execErr to decide exit code
	if rt.term != nil {
		rt.term.Restore()
	}

	// trigger OnShutdown hooks
	rt.CancelCtx()
	waitErr := rt.Wait()

	// log first failure if any
	if execErr != nil && *execErr != nil {
		logs.Errorf("%s error: %v", appName, *execErr)
		if helpHint != "" {
			fmt.Fprintln(os.Stderr, helpHint)
		}
	} else if waitErr != nil {
		logs.Errorf("%s fail reason: %v", appName, waitErr)
	}

	logs.Close()
}
