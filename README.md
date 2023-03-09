# btr

Build Task Runner is a utility that assists in batch running certain tasks
related to building software applications (mostly targeting C++):

- making multi-resolution icons from sets of JPEG/PNG files
- code-generator for embedding binary resources as C++ arrays
- code-generator for embedding images (bitmaps) as C++ sources
- converting SVG files into SVG fonts
- converting SVG fonts to TrueType fonts

This utility executes a list of tasks described in a yaml file which is passed
as a parameter to the `btr` executable.

## Installation

To install a binary release:

- download the file matching your platform here: [Latest release
  binaries](https://github.com/adnsv/btr/releases/latest)
- unzip it into the directory of your choice
- make sure your system path resolves to that directory

To build and install `btr` from sources:

- make sure you have a recent GO compiler installed
- execute `go install github.com/adnsv/btr@latest`

**Note:** When running the `ttf` task (see below), btr is using `svg2ttf`
utility. That requires Node.js installed in your system, then it can be obtained
with: `npm install -g svg2ttf`.

## Project File Structure

The overall structure of the file that btr executes is shown below:

```yaml
# file schema version
version: 0.4.0

# user-defined variables that can be used in the tasks
vars: 
  "key": value
  "myvar": another-value

# a list of tasks
tasks: 
  - name: Displayed name of the task
    type: <one of the predefined task types, see below>
    # task-dependent fields
    source: ${key}
    target: ${myvar}
```

The `version` section specifies the version of the btr software required to run
this file.

The `vars` section defines global variables, key-value pairs, that can be used
within the tasks. Referring to variable values is done with the syntax
`${var-name}`.

The `tasks` section contains the list of tasks that is executed in the order of
appearance. Each task must contain a `type` field that specifies the type of
task and an optional `name` field that will be displayed in the console when the
task is running. The rest of the fields is task-dependent.

**Note** Paths to files and directories specified within the `vars` and `tasks`
sections can be absolute or relative. The relative paths are expanded relative
to the location of the project file.

## `dir` task

The `dir` task allows creating directories within the file system.

| field      | value  | description    |
| ---------- | ------ | -------------- |
| path       | string, optional           | Name of the directory within the file system, may include variables; when this field is omitted, the application will use a temporary directory. |
| if-missing | `create`|`error`, optional | The action taken when the specified directory does not exist (default: `create`). |
| if-exists  | `clean`|`error`, optional  | The action taken when the specified directory already exists (default: no action). |
| var        | string, optional           | Insert the path to the directory into the list of global vars. |

## `file` task

The `file` task allows creating text files.

| field   | value            | description                                                 |
| ------- | ---------------- | ----------------------------------------------------------- |
| target  | string, required | Path to file within the file system, may include variables. |
| content | string, optional | File content.                                               |

## `binpack` task

The `binpack` task allows packing files into binary resources for embedding into
your application. This task code-generates files that you can include in your
application sources to access the data as arrays of bytes.

| field  | value | description |
| ------ | ----- | ----------- |
| source | string or array of strings, required | Paths to files for packing, may include wildcards, double-star `**` for traversing subdirs recursively, and variables. |
| target | map or list of maps, required        | See below.                                                   |

The task can code-generates one or more targets (e.g. hpp/cpp) from the same set
of resources. Each target within the `binpack` task is a map that has the
following fields:

| field   | value            | description                                        |
| ------- | ---------------- | -------------------------------------------------- |
| file    | string, required | Path to the generated file, may include variables. |
| entry   | string, required | A template for each embedded resource.             |
| content | string, required | A template for the content of the file.            |

The produced target files are generated using the `content` template, typically
a multi-line string that includes references to existing global variables. In
addition, a variable named `${entries}` is available that contains the list of
all the resources. 

The `${entries}` variable is composed by concatenating entry fragments, which in
turn are produced with the `entry` template. Within the `entry` template the
following additional variables are available for expansion:

- `${byte-count}` the number of bytes within the resource
- `${byte-content}` a comma-separated list of bytes, hex encoded, e.g. `0x00,
  0x01, ...`
- `${filename}` a stem of the filepath from which the resource is pulled.
- `${ident-cpp}` a string that is obtained from the filename by lower-casing and
  replacing all dots and dashes with underscores, if the result collides with a
  C++ reserved keyword it is postfixed with an additional underscore.

## `svgfont` task

The `svgfont` task composes SVG files into an SVG font. The produced SVG font
then can be converted to produce a true-type font with the `ttf` task that is
composed of the glyphs generated from the original SVG sources.

| field           | value   | description |
| --------------- | ------- | ----------- |
| sources         | string or list of strings, required | Paths to SVG files, may include wildcards, double-star `**` for traversing subdirs recursively, and variables. |
| target          | string, required                    | Path to the generated SVG font file, may include variables.  |
| first-codepoint | string, optional                    | The generated font will associate glyphs with Unicode values in the [`first-codepoint`...`first-codepoint`+`<number-of-glyphs>`) range. Use `U+0000` or `0x000` syntax to specify hexadecimal value, or a plain integer for decimals. Defaults to `U+F000` when this filed is ommited. |
| height          | integer, optional                   | Overall height of the generated font in internal font units, defaults to 512. |
| descent         | integer, optional                   | Descent value for the glyphs in internal font units, defaults to 25% of the height. |
| family          | string, optional                    | Name of the font family; when ommited the family name will be generated from the target by removing its filename extension. |

## `ttf` task

The `ttf` task converts an SVG font into a TrueType font. It uses `svg2ttf`
under the hood.

| field  | value            | description                                            |
| ------ | ---------------- | ------------------------------------------------------ |
| source | string, required | Path to SVG font file, may include variables.          |
| target | string, required | Path to the generated TTF file, may include variables. |

## `glyph-names` task

The `glyph-names` task allows to code-generate a file that maps glyph names to
their Unicode values. In present implementation, it only supports SVG font as
the source for glyph names.

| field  | value                         | description                                   |
| ------ | ----------------------------- | --------------------------------------------- |
| source | string, required              | Path to SVG font file, may include variables. |
| target | map or list of maps, required | See below.                                    |

The task can code-generates one or more targets. Each target within the
`glyph-names` task is a map that has the following fields:

| field   | value            | description                                        |
| ------- | ---------------- | -------------------------------------------------- |
| file    | string, required | Path to the generated file, may include variables. |
| entry   | string, required | A template for each named glyph entry.             |
| content | string, required | A template for the content of the file.            |

The produced target files are generated using the `content` template, typically
a multi-line string that includes references to existing global variables. In
addition, a variable named `${entries}` is available that contains the list of
all the glyphs. 

The `${entries}` variable is composed by concatenating entry fragments, which in
turn are produced with the `entry` template. Within the `entry` template the
following additional variables are available for expansion:

- `${name}` an original glyph name
- `${ident-cpp}` a string that is obtained from the glyph name by replacing all
  dashes with underscores, it will also postfix the string with an undescore to
  avoid collisions with C++ reserved words.

- `${unicode}` a string that contains a Unicode value of the glyph in the
  `U+XXXX` form, where XXXX are hex digits.
- `${unicode}` a string that contains a Unicode value of the glyph in the `XXXX`
  form where XXXX are hex digits.
- `${utf8}` a Unicode glyph value represented as a UTF-8 string.
- `${utf8-escaped-cpp}` a Unicode glyph value represented as a sequence of C++
  escaped code units.

## `glfw-icon` task

Code-generates a C++ file that can be used for embedding multi-resolution GLFW
icon into your application from a set of PNG/JPEG files.

| field  | value | description |
| ------ | ----- | ----------- |
| source | string or a list of strings, required | Paths to PNG/JPEG files, may include wildcards, double-star `**` for traversing subdirs recursively, and variables. |
| target | string, required                      | Path to the generated C++ file, may include variables.       |
| ident  | string, optional                      | A C++ identifier name for the generated constant.            |

## `win32-icon` task

Generates WIN32 `.ico` multi-resolution icon from a set of PNG/JPEG files.

| field  | value | description |
| ------ | ----- | ----------- |
| source | string or a list of strings, required | Paths to PNG/JPEG files, may include wildcards, double-star `**` for traversing subdirs recursively, and variables. |
| target | string, required                      | Path to the generated `.ico` file, may include variables.    |

