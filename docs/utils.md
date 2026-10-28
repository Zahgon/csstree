# Utils

Package `utils`:

- `Keyword(name)` returns a `Descriptor` for a keyword: `Name` (lowercased), `Basename` (without a vendor prefix),
  `Prefix`, `Vendor` and `Custom` (starts with `--`).
- `Property(name)` returns a `Descriptor` for a property name, also recognising the hacks `_`, `*`, `$`, `#`, `+`,
  `&`, `/` and `//` (`Hack`). Custom properties keep their case.
- `VendorPrefix(name, offset)` and `IsCustomProperty(name, offset)`.
- `IdentEncode`/`IdentDecode`, `StringEncode(s, apostrophe)`/`StringDecode`, `URLEncode`/`URLDecode` encode and
  decode identifiers, strings and `url()` tokens with CSS escapes.

`csstree.Clone(node)` deep-copies a node; `csstree.ToPlainObject` and `FromPlainObject` switch children between
lists and arrays.
