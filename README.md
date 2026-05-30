# Pole

**Pole** is a feature flag helper for Go.
It is designed to provide dynamic, file-driven feature flag management with hot-reload support and optional per-field change hooks.

---

# Features

* Generic, type-safe flag configuration using Go generics
* File-based configuration (JSON, YAML, TOML support via extension)
* Automatic hot-reloading via file watcher
* Field-level change detection
* Reflection-based callback system (`onChange` tags)
* Minimal and dependency-light design
* Extensible architecture for additional formats

---

# Installation

```bash
go get github.com/GokselKUCUKSAHİN/pole
```

---

# Supported Formats

| Format | Extensions                   |
| ------ | ---------------------------- |
| JSON   | `.json`                      |
| YAML   | `.yaml`, `.yml`              |
| TOML   | `.toml` (optional extension) |

---

# Quick Start

Define your feature flags as a struct:

```go
type Flags struct {
	NewCheckout bool `yaml:"newCheckout" onChange:"OnNewCheckoutChanged"`
	BetaMode    bool `yaml:"betaMode" onChange:"OnBetaModeChanged"`
}
```

Load flags from a file:

```go
func main() {
	flags, err := pole.Read[Flags]("flags.yaml")
	if err != nil {
		panic(err)
	}

	fmt.Println(flags.BetaMode)
}
```

---

# Hot Reloading

Once loaded, Pole watches the file for changes automatically.
When a change is detected:

* The file is re-parsed
* The previous and new values are compared
* Changed fields trigger their associated callbacks

---

# Change Handlers

You can react to flag changes using method hooks.

### Signature Options

#### No parameters

```go
func (f *Flags) OnBetaModeChanged() {
	fmt.Println("Beta mode updated")
}
```

#### Old value

```go
func (f *Flags) OnBetaModeChanged(old bool) {
	fmt.Println("Previous value:", old)
}
```

#### Old and New values

```go
func (f *Flags) OnBetaModeChanged(old, new bool) {
	fmt.Printf("Beta mode changed: %v → %v\n", old, new)
}
```

---

# Configuration Examples

## JSON

```json
{
  "NewCheckout": true,
  "BetaMode": false
}
```

---

## YAML

```yaml
NewCheckout: true
BetaMode: false
```

---

## TOML

```toml
NewCheckout = true
BetaMode = false
```

---

# How It Works

Pole uses a generic file reader internally that:

1. Loads configuration into a typed struct (`T`)
2. Watches the file for changes
3. On update:

    * Parses the new file
    * Compares old vs new struct values via reflection
    * Triggers `onChange` hooks for modified fields

---

# License

LGPL-2.1
