package client_test

import (
	"path/filepath"
	"testing"

	"github.com/lkingland/faas/client"
	"github.com/lkingland/faas/client/mock"
)

// TestNew ensures that instantiation succeeds or fails as expected.
func TestNew(t *testing.T) {
	// Instantiation with optional explicit service name should succeed.
	_, err := client.New(
		client.WithRoot("./testdata/example.com/admin"))
	if err != nil {
		t.Fatal(err)
	}

	// Instantiation with optional verbosity should succeed.
	_, err = client.New(
		client.WithRoot("./testdata/example.com/admin"),
		client.WithVerbose(true),
	)
	if err != nil {
		t.Fatal(err)
	}

	// Instantiation without an explicit service name, but no derivable service
	// name (because of limiting path recursion) should fail.
	_, err = client.New(
		client.WithDomainSearchLimit(0), // limit ability to derive from path.
	)
	if err == nil {
		t.Fatal("no error generated for unspecified and underivable name")
	}
}

// TestNewWithInterferingFiles asserts that attempting to create a new client rooted
// to a directory with any visible files or any known contentious files (configs) fails.
func TestNewWithInterferingFiles(t *testing.T) {
	// TODO
}

// TestCreate ensures that instantiation completes without error when provided with a
// valid language.  A single client instance services a single Service Function instance
// and as such requires the desired effective DNS for the function.  This is an optional
// parameter, as it is derived from path by default.
func TestCreate(t *testing.T) {
	client, err := client.New(
		client.WithRoot("./testdata/example.com/admin"),
		client.WithInitializer(mock.NewInitializer()),
	) // be explicit rather than path-derive
	if err != nil {
		t.Fatal(err)
	}

	// create a Function Service call missing language should error
	if err := client.Create(""); err == nil {
		t.Fatal("missing language did not generate error")
	}

	// create a Function Service call witn an unsupported language should bubble
	// the error generated by the underlying initializer.
	if err := client.Create("cobol"); err == nil {
		t.Fatal("unsupported language did not generate error")
	}

	// A supported langauge should not error.
	if err := client.Create("go"); err != nil {
		t.Fatal(err)
	}
}

// TestCreateDelegeates ensures that a call to Create invokes the Service Function
// Initializer, Builder, Pusher and Deployer with expected parameters.
func TestCreateDelegates(t *testing.T) {
	var (
		path        = "testdata/example.com/admin" // .. in which to initialize
		name        = "admin.example.com"          // expected to be derived
		image       = "my.hub/user/imagestamp"     // expected image
		route       = "https://admin.example.com/" // expected final route
		initializer = mock.NewInitializer()
		builder     = mock.NewBuilder()
		pusher      = mock.NewPusher()
		deployer    = mock.NewDeployer()
	)

	client, err := client.New(
		client.WithRoot(path),               // set function root
		client.WithInitializer(initializer), // will receive the final value
		client.WithBuilder(builder),         // builds an image
		client.WithPusher(pusher),           // pushes images to a registry
		client.WithDeployer(deployer),       // deploys images as a running service
	)
	if err != nil {
		t.Fatal(err)
	}

	// Register function delegates on the mocks which validate assertions
	// -------------

	// The initializer should receive the name expected from the path,
	// the passed language, and an absolute path to the funciton soruce.
	initializer.InitializeFn = func(name, language, path string) error {
		if name != "admin.example.com" {
			t.Fatalf("initializer expected name 'admin.example.com', got '%v'", name)
		}
		if language != "go" {
			t.Fatalf("initializer expected language 'go', got '%v'", language)
		}
		expectedPath, err := filepath.Abs("./testdata/example.com/admin")
		if err != nil {
			t.Fatal(err)
		}
		if path != expectedPath {
			t.Fatalf("initializer expected path '%v', got '%v'", expectedPath, path)
		}
		return nil
	}

	// The builder should be invoked with a service name and path to its source
	// function code.  For this test, it is a name derived from the test path.
	// An example image name is returned.
	builder.BuildFn = func(name2, path2 string) (string, error) {
		if name != name {
			t.Fatalf("builder expected name %v, got '%v'", name, name2)
		}
		expectedPath, err := filepath.Abs(path)
		if err != nil {
			t.Fatal(err)
		}
		if path2 != expectedPath {
			t.Fatalf("builder expected path '%v', got '%v'", expectedPath, path)
		}
		// The final image name will be determined by the builder implementation,
		// but whatever it is (in this case fabricarted); it should be returned
		// and later provided to the pusher.
		return image, nil
	}

	// The pusher should be invoked with the image to push.
	pusher.PushFn = func(image2 string) error {
		if image2 != image {
			t.Fatalf("pusher expected image '%v', got '%v'", image, image2)
		}
		// image of given name wouold be pushed to the configured registry.
		return nil
	}

	// The deployer should be invoked with the service name and image, and return
	// the final accessible address.
	deployer.DeployFn = func(name2, image2 string) (address string, err error) {
		if name2 != name {
			t.Fatalf("deployer expected name '%v', got '%v'", name, name2)
		}
		if image2 != image {
			t.Fatalf("deployer expected image '%v', got '%v'", image, image2)
		}
		// service of given name would be deployed using the given image and
		// allocated route returned.
		return route, nil
	}

	// Invocation
	// -------------

	// Invoke the creation, triggering the function delegates, and
	// perform follow-up assertions that the functions were indeed invoked.
	if err := client.Create("go"); err != nil {
		t.Fatal(err)
	}

	// Confirm that each delegate was invoked.
	if !initializer.InitializeInvoked {
		t.Fatal("initializer was not invoked")
	}
	if !builder.BuildInvoked {
		t.Fatal("builder was not invoked")
	}
	if !pusher.PushInvoked {
		t.Fatal("pusher was not invoked")
	}
	if !deployer.DeployInvoked {
		t.Fatal("deployer was not invoked")
	}
}

// TestCreateLocal ensures that when set to local-only mode, Create only invokes
// the initializer and builder.
func TestCreateLocal(t *testing.T) {
	var (
		path        = "testdata/example.com/admin"
		initializer = mock.NewInitializer()
		builder     = mock.NewBuilder()
		pusher      = mock.NewPusher()
		deployer    = mock.NewDeployer()
		dnsProvider = mock.NewDNSProvider()
	)

	client, err := client.New(
		client.WithRoot(path),               // set function root
		client.WithInitializer(initializer), // will receive the final value
		client.WithBuilder(builder),         // builds an image
		client.WithPusher(pusher),           // pushes images to a registry
		client.WithDeployer(deployer),       // deploys images as a running service
		client.WithDNSProvider(dnsProvider), // will receive the final value
	)
	if err != nil {
		t.Fatal(err)
	}
	// Set the client to local-only mode
	client.SetLocal(true)

	// Create a new Service Function
	if err := client.Create("go"); err != nil {
		t.Fatal(err)
	}
	// Ensure that none of the remote delegates were invoked
	if pusher.PushInvoked {
		t.Fatal("Push invoked in local mode.")
	}
	if deployer.DeployInvoked {
		t.Fatal("Deploy invoked in local mode.")
	}
	if dnsProvider.ProvideInvoked {
		t.Fatal("DNS provider invoked in local mode.")
	}

}

// TestCreateDomain ensures that the effective domain is dervied from
// directory structure.  See the unit tests for pathToDomain for details.
func TestCreateDomain(t *testing.T) {
	// the mock dns provider does nothing but receive the caluclated
	// domain name via it's Provide(domain) method, which is the value
	// being tested here.
	dnsProvider := mock.NewDNSProvider()

	client, err := client.New(
		client.WithRoot("./testdata/example.com/admin"), // set function root
		client.WithDomainSearchLimit(1),                 // Limit recursion to one level
		client.WithDNSProvider(dnsProvider),             // will receive the final value
	)
	if err != nil {
		t.Fatal(err)
	}

	if err := client.Create("go"); err != nil {
		t.Fatal(err)
	}
	if !dnsProvider.ProvideInvoked {
		t.Fatal("dns provider was not invoked")
	}
	if dnsProvider.NameRequested != "admin.example.com" {
		t.Fatalf("expected 'example.com', got '%v'", dnsProvider.NameRequested)
	}
}

// TestCreateSubdomain ensures that a subdirectory is interpreted as a subdomain
// when calculating final domain.  See the unit tests for pathToDomain for the
// details and edge cases of this caluclation.
func TestCreateSubdomain(t *testing.T) {
	dnsProvider := mock.NewDNSProvider()
	client, err := client.New(
		client.WithRoot("./testdata/example.com/admin"),
		client.WithDomainSearchLimit(2),
		client.WithDNSProvider(dnsProvider),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Create("go"); err != nil {
		t.Fatal(err)
	}
	if !dnsProvider.ProvideInvoked {
		t.Fatal("dns provider was not invoked")
	}
	if dnsProvider.NameRequested != "admin.example.com" {
		t.Fatalf("expected 'admin.example.com', got '%v'", dnsProvider.NameRequested)
	}
}

// TestRun ensures that the runner is invoked with the absolute path requested.
func TestRun(t *testing.T) {
	root := "./testdata/example.com/admin"
	runner := mock.NewRunner()
	client, err := client.New(
		client.WithRoot(root),
		client.WithRunner(runner),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Run(); err != nil {
		t.Fatal(err)
	}
	if !runner.RunInvoked {
		t.Fatal("run did not invoke the runner")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		t.Fatal(err)
	}
	if runner.RootRequested != absRoot {
		t.Fatalf("expected path '%v', got '%v'", absRoot, runner.RootRequested)
	}
}

// TestRemove ensures that the remover is invoked with the name of the
// client's associated service function.
func TestRemove(t *testing.T) {
	var (
		root    = "./testdata/example.com/admin"
		name    = "admin.example.com"
		remover = mock.NewRemover()
	)
	client, err := client.New(
		client.WithRoot(root),
		client.WithRemover(remover))
	if err != nil {
		t.Fatal(err)
	}
	remover.RemoveFn = func(name2 string) error {
		if name2 != name {
			t.Fatalf("remover expected name '%v' got '%v'", name, name2)
		}
		return nil
	}
	// Call remove with no explicit name, expecting default to be the
	// assocaite of the client instance
	if err := client.Remove(""); err != nil {
		t.Fatal(err)
	}
}

// TestRemoveExplicit ensures that a call to remove an explicit name, which
// may differ from the service function the client is associated wtith, is
// respected and passed along to the concrete remover implementation.
func TestRemoveExplicit(t *testing.T) {
	var (
		root    = "./testdata/example.com/admin"
		name    = "www.example.com" // Differs from that derived from root.
		remover = mock.NewRemover()
	)
	client, err := client.New(
		client.WithRoot(root),
		client.WithRemover(remover))
	if err != nil {
		t.Fatal(err)
	}
	remover.RemoveFn = func(name2 string) error {
		if name2 != name {
			t.Fatalf("remover expected name '%v' got '%v'", name, name2)
		}
		return nil
	}
	// Call remove with an explicit name which differs from that associated
	// to the current client instance.
	if err := client.Remove(name); err != nil {
		t.Fatal(err)
	}
}
