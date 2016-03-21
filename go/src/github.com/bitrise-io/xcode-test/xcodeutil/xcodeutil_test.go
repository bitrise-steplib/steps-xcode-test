package xcodeutil

import (
	"testing"

	"github.com/bitrise-io/xcode-test/models"
)

func TestIsOsVersionGreater(t *testing.T) {
	t.Log("iOS 9.0 < iOS 9.1")
	{
		greater, err := isOsVersionGreater("iOS 9.0", "iOS 9.1")
		if err != nil {
			t.Fatalf("Expected (nil), actual(%s)", err)
		}

		if greater {
			t.Fatalf("Expected (false), actual(%s)", greater)
		}
	}

	t.Log("iOS 9.1 > iOS 9.0")
	{
		greater, err := isOsVersionGreater("iOS 9.1", "iOS 9.0")
		if err != nil {
			t.Fatalf("Expected (nil), actual(%s)", err)
		}

		if !greater {
			t.Fatalf("Expected (true), actual(%s)", greater)
		}
	}

	t.Log("iOS 9.1 > iOS 8.3")
	{
		greater, err := isOsVersionGreater("iOS 9.1", "iOS 8.3")
		if err != nil {
			t.Fatalf("Expected (nil), actual(%s)", err)
		}

		if !greater {
			t.Fatalf("Expected (true), actual(%s)", greater)
		}
	}
}

func TestGetLatestOsVersion(t *testing.T) {
	t.Log("1 OS Version")
	{
		allSimIDsGroupedBySimVersion := models.SimulatorsGroupedByIOSVersionsModel{
			"iOS 9.0": []models.SimInfoModel{
				models.SimInfoModel{
					Name: "iPhone 6",
				},
				models.SimInfoModel{
					Name: "iPhone 6 Plus",
				},
				models.SimInfoModel{
					Name: "iPad 2",
				},
			},
		}

		latestOsVersion, err := getLatestOsVersion("iOS", "iPhone 6", allSimIDsGroupedBySimVersion)
		if err != nil {
			t.Fatalf("Expected (nil), actual(%s)", err)
		}

		if latestOsVersion != "iOS 9.0" {
			t.Fatalf("Expected (iOS 9.0), actual(%s)", latestOsVersion)
		}
	}

	t.Log("Multiple OS version")
	{
		allSimIDsGroupedBySimVersion := models.SimulatorsGroupedByIOSVersionsModel{
			"iOS 9.2": []models.SimInfoModel{
				models.SimInfoModel{
					Name: "iPhone 6 Plus",
				},
			},
			"iOS 9.0": []models.SimInfoModel{
				models.SimInfoModel{
					Name: "iPhone 6",
				},
			},
			"iOS 9.1": []models.SimInfoModel{
				models.SimInfoModel{
					Name: "iPhone 6",
				},
			},
		}

		latestOsVersion, err := getLatestOsVersion("iOS", "iPhone 6", allSimIDsGroupedBySimVersion)
		if err != nil {
			t.Fatalf("Expected (nil), actual(%s)", err)
		}

		if latestOsVersion != "iOS 9.1" {
			t.Fatalf("Expected (iOS 9.1), actual(%s)", latestOsVersion)
		}
	}

	t.Log("Multiple OS version")
	{
		allSimIDsGroupedBySimVersion := models.SimulatorsGroupedByIOSVersionsModel{
			"iOS 9.2": []models.SimInfoModel{
				models.SimInfoModel{
					Name: "iPhone 6",
				},
			},
			"iOS 8.3": []models.SimInfoModel{
				models.SimInfoModel{
					Name: "iPhone 6",
				},
				models.SimInfoModel{
					Name: "iPhone 5",
				},
				models.SimInfoModel{
					Name: "iPhone 4",
				},
			},
		}

		latestOsVersion, err := getLatestOsVersion("iOS", "iPhone 6", allSimIDsGroupedBySimVersion)
		if err != nil {
			t.Fatalf("Expected (nil), actual(%s)", err)
		}

		if latestOsVersion != "iOS 9.2" {
			t.Fatalf("Expected (iOS 9.2), actual(%s)", latestOsVersion)
		}
	}

	t.Log("Multiple OS version")
	{
		allSimIDsGroupedBySimVersion := models.SimulatorsGroupedByIOSVersionsModel{
			"iOS 9.2": []models.SimInfoModel{
				models.SimInfoModel{
					Name: "iPhone 6",
				},
			},
			"iOS 8.3": []models.SimInfoModel{
				models.SimInfoModel{
					Name: "iPhone 6",
				},
				models.SimInfoModel{
					Name: "iPhone 5",
				},
				models.SimInfoModel{
					Name: "iPhone 4",
				},
			},
		}

		latestOsVersion, err := getLatestOsVersion("iOS", "iPhone 5", allSimIDsGroupedBySimVersion)
		if err != nil {
			t.Fatalf("Expected (nil), actual(%s)", err)
		}

		if latestOsVersion != "iOS 8.3" {
			t.Fatalf("Expected (iOS 8.3), actual(%s)", latestOsVersion)
		}
	}

	t.Log("Multiple OS version -- device not exist")
	{
		allSimIDsGroupedBySimVersion := models.SimulatorsGroupedByIOSVersionsModel{
			"iOS 9.2": []models.SimInfoModel{
				models.SimInfoModel{
					Name: "iPhone 6",
				},
			},
			"iOS 8.3": []models.SimInfoModel{
				models.SimInfoModel{
					Name: "iPhone 6",
				},
				models.SimInfoModel{
					Name: "iPhone 5",
				},
				models.SimInfoModel{
					Name: "iPhone 4",
				},
			},
		}

		latestOsVersion, err := getLatestOsVersion("iOS", "iPhone 6 Plus", allSimIDsGroupedBySimVersion)
		if err == nil {
			t.Fatalf("Expected (nil), actual(%s)", err)
		}

		if latestOsVersion != "" {
			t.Fatalf("Expected (), actual(%s)", latestOsVersion)
		}
	}
}
