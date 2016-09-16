package command

import "testing"

func TestPrintableCommandArgs(t *testing.T) {
	t.Log("Printable command test")
	{
		orgCmdArgs := []string{
			"xcodebuild", "-project", "MyProj.xcodeproj", "-scheme", "MyScheme",
			"build", "test",
			"-destination", "platform=iOS Simulator,name=iPhone 6,OS=latest",
			"-sdk", "iphonesimulator",
		}
		resStr := PrintableCommandArgs(orgCmdArgs)
		expectedStr := `xcodebuild "-project" "MyProj.xcodeproj" "-scheme" "MyScheme" "build" "test" "-destination" "platform=iOS Simulator,name=iPhone 6,OS=latest" "-sdk" "iphonesimulator"`

		if resStr != expectedStr {
			t.Log("PrintableCommandArgs failed to generate the expected string!")
			t.Logf(" -> expectedStr: %s", expectedStr)
			t.Logf(" -> resStr: %s", resStr)
			t.Fatalf("Expected string does not match the generated string. Original args: (%#v)", orgCmdArgs)
		}
	}
}

func TestPrintableCommandArgsWithEnvs(t *testing.T) {
	t.Log("Printable command test - with env vars")
	{
		orgCmdArgs := []string{
			"xcodebuild", "-project", "MyProj.xcodeproj", "-scheme", "MyScheme",
			"build", "test",
			"-destination", "platform=iOS Simulator,name=iPhone 6,OS=latest",
			"-sdk", "iphonesimulator",
		}
		resStr := PrintableCommandArgsWithEnvs(orgCmdArgs, []string{"NSUnbufferedIO=YES"})
		expectedStr := `env "NSUnbufferedIO=YES" xcodebuild "-project" "MyProj.xcodeproj" "-scheme" "MyScheme" "build" "test" "-destination" "platform=iOS Simulator,name=iPhone 6,OS=latest" "-sdk" "iphonesimulator"`

		if resStr != expectedStr {
			t.Log("PrintableCommandArgsWithEnvs failed to generate the expected string!")
			t.Logf(" -> expectedStr: %s", expectedStr)
			t.Logf(" -> resStr: %s", resStr)
			t.Fatalf("Expected string does not match the generated string. Original args: (%#v)", orgCmdArgs)
		}
	}
}
