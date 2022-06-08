# btr

Build Task Runner is a small utility that assists in batch running certain tasks
related to building software applications (mostly targeting C++):

- making Win32 icon resources from PNGs
- codegen for embedding binary resources as C++ arrays
- codegen for embedding images (bitmaps) as C++ sources
- converting SVG files into SVG font (also to TTF with svg2ttf utility)

This utility executes a list of tasks described in json configuration file.

## Config File Structure

The root of the a JSON object:

```json
{
    "version": "1.0.0",
    "codegen": {
        "top-matter": {
            "svg": [
                "<svg xmlns='http://www.w3.org/2000/svg'>", 
                "<!-- DO NOT EDIT: Generated file -->", 
            ],
            "hpp": [
                "#pragma once",
                "",
                "// DO NOT EDIT: Generated file",
                "// clang-format off",
                ""
            ],
            "cpp": [
                "// DO NOT EDIT: Generated file",
                "// clang-format off",
                ""
            ]
        },
        "bottom-matter": {
            "svg": [
                "</svg>"
            ]
        }
    },
    "tasks": [
        {
            "label": "First Task", // will be printed on console when task is executed
            "type": "<task-type>", // one of the predefined tasks, see below
            // task fields...
        },
        {
            "label": "Second Task",
            "type": "<task-type>",
            // task fields ...
        },
        // other tasks
    ]
}
```

## SVG Font Generator

### Composing SVG Files to SVG Font

Task type: `svgfont.make`

Fields:

- `target`: a filepath to generated svg font
- `source` or `sources`: a masked path (or a list of paths) specifying where to
  find svg files
- `font`.`firstCodepoint`: first codepoint value (unicode)
- `font`.`height`: height of the generated font in svg units
- `font`.`descent`: descent that defines baseline alignment in svg units
- `font`.`family`: optional family name for the generated font

Example:

```json
{
    "label": "Make SVG font",
    "type": "svgfont.make",
    "source": "./icons/*.svg",
    "target": "./icons.svg",
    "font": {
        "firstCodepoint": "U+F000",
        "height": 512,
        "descent": 128
    }
}
```

### Generating C++ Header for Glyph Names

Task type: `svgfont.hpp`

Fields:

- `target`: name of the generated `.hpp` file
- `source`: svg font file, normally should match `target` field from a
  preceeding `svgfont.make` task
- `codegen`.`namespace`: namespace for the generated file 
- `codegen`.`typename`: specify custom type to use for glyph constants

Example:

```json
{
    "label": "Make C++ header for SVG font",
    "type": "svgfont.hpp",
    "source": "./icons.svg",
    "target": "./icons.hpp",
    "codegen": {
        "namespace": "icon",
        "typename": "constexpr char const*"
    }
}
```        

### Converting SVG font to TTF

Task type: `svgfont.ttf`

Fields:

- `target`: name of the generated ttf font
- `source`: svg font file, normally should match `target` field from a
  preceeding `svgfont.make` task

Example:

```json
{
    "label": "Convert SVG font to TTF",
    "type": "svgfont.ttf",
    "source": "./icons.svg",
    "target": "./icons.ttf"
}
```

## Packing Binary Resources

Task type: `binpack.c++`

Fields:

- `source`: source filename
- `targets`: specify a pair of .hpp and .cpp filenames
- `codegen`.`namespace`: namespace for the generated file 
- `codegen`.`typename`: carrier type (`byte`, `unsigned char`, etc...)
- `codegen`.`top-matter`: additional content to inject into the generated files

Example:

```json
{
    "label": "Generate C++ binary resource for TTF font",
    "type": "binpack.c++",
    "source": "./icons.ttf",
    "targets": [
        "./icon-resource.hpp",
        "./icon-resource.cpp"
    ],
    "codegen": {
        "namespace": "app::resource",
        "typename": "byte",
        "top-matter": {
            "hpp": [
                "#include <array>",
                ""
            ],
            "cpp": [
                "#include \"icon-resource.hpp\"",
                ""
            ]
        }
    }
}
```

## Embedding Image Resources

BTR features a code generator that embeds images (jpeg, png) into a C++ source code.

A typical routine would involve generation of a common header file that contains
a struct definition for an image header, followed up by packing image resources.


### Writing Common Image Resource Header

A codegenerator that 

Task type: `imgpack.c++.types`

```json
{
    "label": "Generate C++ image resource type header",
    "type": "imgpack.c++.types",
    "target": "./img-types.hpp",
    "codegen": {
        "namespace": "app::resource",
        "typename": "image",
        "top-matter": {
            "hpp": [
                "#include <cstddef>",
                "#include <string_view>",
                ""
            ]
        }
    }
}
```

### Packing Image Resources

Task type: `imgpack.c++`

Fields:

- `targets`: a pair of hpp/cpp files to generate
- `source` or `sources`: specify input files (jpeg or png)
- `format`: one of the following:
    - `prgba` - premultiplied array of RGBA values
    - `nrgba` - non-premultiplied array of RGBA values
    - `png` - image data packed into a png stream

Example:

```json
{
    "label": "Pack app-icon images into a C++ binary resource for GLFW",
    "type": "imgpack.c++",
    "sources": [
        "./app-icon/icon-16.png",
        "./app-icon/icon-24.png",
        "./app-icon/icon-32.png",
        "./app-icon/icon-48.png",
        "./app-icon/icon-64.png"
    ],
    "targets": [
        "./app-icon-resource.hpp",
        "./app-icon-resource.cpp"
    ],
    "format": "nrgba",
    "codegen": {
        "namespace": "app::resource",
        "typename": "resource::image",
        "top-matter": {
            "hpp": [
                "#include <array>",
                "#include \"./img-types.hpp\"",
                ""
            ],
            "cpp": [
                "#include \"app-icon-resource.hpp\"",
                ""
            ]
        }
    }
}
```

## Making Win32 Icon

Takes image files, typically an application icon at different resolutions, and
produce a win32 icon (.ico) file.

Task type: `icon.win32`

Fields:

- `sources`: input image files
- `target`: path to the generated .ico file

Example:

```json
{
    "label": "Make Win32 Icon",
    "type": "icon.win32",
    "sources": [
        "./app-icon/icon-16.png",
        "./app-icon/icon-24.png",
        "./app-icon/icon-32.png",
        "./app-icon/icon-48.png",
        "./app-icon/icon-64.png",
        "./app-icon/icon-128.png"
    ],
    "target": "./win32.ico"
}
```

