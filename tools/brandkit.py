#!/usr/bin/env python3
"""brandkit - pull a palette out of CSS/screenshots and make it terminal-safe.

  brandkit.py css   <file...>          extract CSS custom properties that hold colors
  brandkit.py image <file...>          extract dominant colors from a screenshot
  brandkit.py term  <hex...>           convert brand colors to terminal-safe tiers
  brandkit.py gogen <name=hex...>      emit a Go theme using lipgloss.CompleteColor

The `term` step is the one that matters. Brand palettes are drawn for a white
page with generous whitespace; a terminal is dense text on a background you do
not control. A brand color dropped in unchanged is very often unreadable.
"""
import colorsys
import math
import re
import sys
from collections import Counter

# ---------------------------------------------------------------- color math

def hex2rgb(h):
    h = h.lstrip("#")
    if len(h) == 3:
        h = "".join(c * 2 for c in h)
    return tuple(int(h[i:i + 2], 16) for i in (0, 2, 4))


def rgb2hex(rgb):
    return "#%02x%02x%02x" % tuple(max(0, min(255, round(c))) for c in rgb)


def _srgb_to_linear(c):
    c /= 255.0
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def _linear_to_srgb(c):
    v = 12.92 * c if c <= 0.0031308 else 1.055 * (c ** (1 / 2.4)) - 0.055
    return v * 255.0


def rgb2oklab(rgb):
    r, g, b = (_srgb_to_linear(c) for c in rgb)
    l = 0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b
    m = 0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b
    s = 0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b
    l_, m_, s_ = (math.copysign(abs(v) ** (1 / 3), v) for v in (l, m, s))
    return (
        0.2104542553 * l_ + 0.7936177850 * m_ - 0.0040720468 * s_,
        1.9779984951 * l_ - 2.4285922050 * m_ + 0.4505937099 * s_,
        0.0259040371 * l_ + 0.7827717662 * m_ - 0.8086757660 * s_,
    )


def oklab2rgb(lab):
    L, a, b = lab
    l_ = L + 0.3963377774 * a + 0.2158037573 * b
    m_ = L - 0.1055613458 * a - 0.0638541728 * b
    s_ = L - 0.0894841775 * a - 1.2914855480 * b
    l, m, s = (v ** 3 for v in (l_, m_, s_))
    return (
        _linear_to_srgb(4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s),
        _linear_to_srgb(-1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s),
        _linear_to_srgb(-0.0041960863 * l - 0.7034186147 * m + 1.7076147010 * s),
    )


def relative_luminance(rgb):
    def f(c):
        c /= 255.0
        return c / 12.92 if c <= 0.03928 else ((c + 0.055) / 1.055) ** 2.4
    r, g, b = (f(c) for c in rgb)
    return 0.2126 * r + 0.7152 * g + 0.0722 * b


def contrast(rgb1, rgb2):
    l1, l2 = relative_luminance(rgb1), relative_luminance(rgb2)
    lo, hi = sorted((l1, l2))
    return (hi + 0.05) / (lo + 0.05)


def fix_contrast(rgb, bg, target=4.5):
    """Nudge lightness in OKLCH until the color is legible on bg.

    Hue and chroma are held fixed, so the result still reads as the same brand
    color; only its lightness moves. Doing this in HSL instead visibly shifts
    hue, which is why OKLab is worth the extra arithmetic.
    """
    if contrast(rgb, bg) >= target:
        return rgb, False
    L, a, b = rgb2oklab(rgb)
    C = math.hypot(a, b)
    H = math.atan2(b, a)
    # Move away from the background: lighter on dark, darker on light.
    direction = 1 if relative_luminance(bg) < 0.18 else -1
    best = rgb
    lo, hi = (L, 1.0) if direction > 0 else (0.0, L)
    for _ in range(40):
        mid = (lo + hi) / 2
        cand = oklab2rgb((mid, C * math.cos(H), C * math.sin(H)))
        cand = tuple(max(0, min(255, c)) for c in cand)
        if contrast(cand, bg) >= target:
            best = cand
            if direction > 0:
                hi = mid
            else:
                lo = mid
        else:
            if direction > 0:
                lo = mid
            else:
                hi = mid
    return tuple(round(c) for c in best), True


# ------------------------------------------------------------ xterm palettes

XTERM16 = [
    (0, 0, 0), (128, 0, 0), (0, 128, 0), (128, 128, 0),
    (0, 0, 128), (128, 0, 128), (0, 128, 128), (192, 192, 192),
    (128, 128, 128), (255, 0, 0), (0, 255, 0), (255, 255, 0),
    (0, 0, 255), (255, 0, 255), (0, 255, 255), (255, 255, 255),
]

def xterm256():
    pal = list(XTERM16)
    levels = [0, 95, 135, 175, 215, 255]
    for r in levels:
        for g in levels:
            for b in levels:
                pal.append((r, g, b))
    for i in range(24):
        v = 8 + 10 * i
        pal.append((v, v, v))
    return pal

PAL256 = xterm256()


def nearest(rgb, palette, lo=0, hi=None):
    """Nearest palette index by OKLab distance (perceptual, unlike raw RGB)."""
    target = rgb2oklab(rgb)
    hi = len(palette) if hi is None else hi
    best, bestd = lo, float("inf")
    for i in range(lo, hi):
        c = rgb2oklab(palette[i])
        d = sum((a - b) ** 2 for a, b in zip(target, c))
        if d < bestd:
            best, bestd = i, d
    return best


# ------------------------------------------------------------------ extract

HEX_RE = re.compile(r"#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})\b")
VAR_RE = re.compile(r"(--[\w-]+)\s*:\s*([^;{}]+)")
FUNC_RE = re.compile(r"\b(?:rgba?|hsla?|oklch)\([^)]*\)")


def cmd_css(paths):
    found = {}
    for p in paths:
        text = sys.stdin.read() if p == "-" else open(p, encoding="utf-8", errors="ignore").read()
        for name, val in VAR_RE.findall(text):
            val = val.strip()
            if HEX_RE.fullmatch(val) or FUNC_RE.fullmatch(val):
                found.setdefault(name, val)
    for name, val in found.items():
        swatch = ""
        if HEX_RE.fullmatch(val):
            r, g, b = hex2rgb(val)
            swatch = f"\x1b[48;2;{r};{g};{b}m   \x1b[0m"
        print(f"  {swatch} {name:36} {val}")
    print(f"\n  {len(found)} color variables", file=sys.stderr)


def cmd_image(paths, k=8):
    from PIL import Image
    import numpy as np
    from sklearn.cluster import KMeans

    for p in paths:
        im = Image.open(p).convert("RGB")
        # NEAREST, not the default bicubic: interpolation invents blended
        # colors along every edge, and those blends pollute the clusters.
        im.thumbnail((400, 400), Image.NEAREST)
        arr = np.array(im).reshape(-1, 3)

        km = KMeans(n_clusters=k, n_init=10, random_state=0).fit(arr)
        counts = Counter(km.labels_)
        total = sum(counts.values())

        print(f"\n  {p}")
        for idx, n in counts.most_common():
            rgb = tuple(int(v) for v in km.cluster_centers_[idx])
            pct = 100 * n / total
            sw = f"\x1b[48;2;{rgb[0]};{rgb[1]};{rgb[2]}m      \x1b[0m"
            print(f"   {sw}  {rgb2hex(rgb)}  {pct:5.1f}%")


DARK_BG = hex2rgb("#1e1e1e")   # a typical dark terminal
LIGHT_BG = hex2rgb("#ffffff")  # a typical light terminal


# OKLCH hue anchors for the six chromatic ANSI slots, in degrees.
HUE_SLOTS = [(29, 1), (110, 3), (145, 2), (195, 6), (264, 4), (328, 5)]


def semantic_ansi16(rgb):
    """Map a color to an ANSI 0-15 slot by hue, not by distance.

    Nearest-neighbour in RGB or even OKLab picks grey for a soft purple,
    because the xterm 16-color primaries are fully saturated and a muted
    brand color is numerically closer to grey than to any of them. At this
    tier we want the *name* of the color preserved, not its coordinates.
    """
    L, a, b = rgb2oklab(rgb)
    C = math.hypot(a, b)
    if C < 0.04:
        return 8 if L < 0.7 else 7
    H = math.degrees(math.atan2(b, a)) % 360
    slot = min(HUE_SLOTS, key=lambda s: min(abs(H - s[0]), 360 - abs(H - s[0])))[1]
    return slot + 8 if L > 0.62 else slot


def cmd_term(hexes):
    print(f"  {'brand':9} {'on dark':22} {'on light':22} {'256':>4} {'16':>3}")
    print("  " + "-" * 64)
    for h in hexes:
        rgb = hex2rgb(h)
        dark, dfix = fix_contrast(rgb, DARK_BG)
        light, lfix = fix_contrast(rgb, LIGHT_BG)
        i256 = nearest(dark, PAL256, lo=16)
        i16 = semantic_ansi16(dark)

        def cell(c, bg, fixed):
            r, g, b = c
            br, bg_, bb = bg
            body = f"\x1b[48;2;{br};{bg_};{bb}m\x1b[38;2;{r};{g};{b}m Aa \x1b[0m"
            return f"{body} {rgb2hex(c)} {contrast(c, bg):4.1f}{'*' if fixed else ' '}"

        print(f"  {h:9} {cell(dark, DARK_BG, dfix)}  {cell(light, LIGHT_BG, lfix)} "
              f"{i256:4} {i16:3}")
    print("\n  * = lightness adjusted to reach 4.5:1; hue and chroma preserved",
          file=sys.stderr)


def cmd_gogen(pairs):
    print("// Code generated by brandkit.py. DO NOT EDIT.\n")
    print("package main\n")
    print('import "github.com/charmbracelet/lipgloss"\n')
    print("// Brand colors resolved per color profile. CompleteColor disables")
    print("// lipgloss's automatic degradation, so each tier is chosen")
    print("// deliberately rather than derived by nearest-neighbour at runtime.")
    print("var brand = struct {")
    names = []
    for pair in pairs:
        n, _, _h = pair.partition("=")
        names.append(n)
        print(f"\t{n.title()} lipgloss.CompleteAdaptiveColor")
    print("}{")
    for pair in pairs:
        n, _, h = pair.partition("=")
        rgb = hex2rgb(h)
        dark, _ = fix_contrast(rgb, DARK_BG)
        light, _ = fix_contrast(rgb, LIGHT_BG)
        d256, d16 = nearest(dark, PAL256, 16), semantic_ansi16(dark)
        l256, l16 = nearest(light, PAL256, 16), semantic_ansi16(light)
        print(f"\t{n.title()}: lipgloss.CompleteAdaptiveColor{{")
        print(f'\t\tDark:  lipgloss.CompleteColor{{TrueColor: "{rgb2hex(dark)}", '
              f'ANSI256: "{d256}", ANSI: "{d16}"}},')
        print(f'\t\tLight: lipgloss.CompleteColor{{TrueColor: "{rgb2hex(light)}", '
              f'ANSI256: "{l256}", ANSI: "{l16}"}},')
        print("\t},")
    print("}")


if __name__ == "__main__":
    if len(sys.argv) < 3:
        print(__doc__)
        sys.exit(1)
    cmd, args = sys.argv[1], sys.argv[2:]
    {"css": cmd_css, "image": cmd_image, "term": cmd_term, "gogen": cmd_gogen}[cmd](args)
