#!/usr/bin/env python3
"""生成 AI盘译 的应用图标（build/appicon.png 与 build/windows/icon.ico）。

设计：深海军蓝圆角方块 + 三根上升蜡烛（空心蓝 / 浅蓝 / 琥珀）+ 右上角四点火花（代表 AI）。
琥珀色那根是"上涨"，深色底让它在任务栏与深色主题下都清晰；16x16 时仍能看出"上升柱+亮点"。

重新生成（需要 Pillow）：
    pip install pillow
    python scripts/gen-appicon.py

一次生成三个文件，缺一个都会出现"有的地方是新图标、有的地方还是旧的"：
- build/appicon.png        → macOS .app 图标（wails 据此生成 iconfile.icns）、
                             //go:embed 为 icon → data.SetAppIcon（AI 分享图里的头像）
- build/windows/icon.ico   → Windows exe 的资源图标（资源管理器/任务栏，多尺寸含 256）
- build/app.ico            → //go:embed 为 icon2 → 系统托盘图标与各平台对话框图标
"""
import math
import os
import sys

from PIL import Image, ImageChops, ImageDraw, ImageFilter

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BUILD = os.path.join(REPO, "build")

SS = 2                      # 超采样倍数（2048 画布 → 1024 主图）
S = 1024 * SS
K = S / 1024.0

ICO_SIZES = [16, 24, 32, 48, 64, 128, 256]


def _px(*vals):
    return tuple(round(v * K) for v in vals)


def _lerp(a, b, t):
    return tuple(round(a[i] + (b[i] - a[i]) * t) for i in range(3))


def _tile(top=(8, 24, 43), bottom=(14, 58, 92), margin=46, radius=224):
    """圆角方块 + 垂直渐变底。返回 (图像, 圆角蒙版)。"""
    grad = Image.new("RGB", (S, S))
    d = ImageDraw.Draw(grad)
    for y in range(S):
        d.line([(0, y), (S, y)], fill=_lerp(top, bottom, y / (S - 1)))
    mask = Image.new("L", (S, S), 0)
    ImageDraw.Draw(mask).rounded_rectangle(
        _px(margin, margin, 1024 - margin, 1024 - margin),
        radius=round(radius * K), fill=255)
    img = Image.new("RGBA", (S, S), (0, 0, 0, 0))
    img.paste(grad, (0, 0), mask)
    return img, mask


def _sparkle(draw, cx, cy, outer, inner, color, points=4, rotation=-90):
    pts = []
    step = 360 / (points * 2)
    for i in range(points * 2):
        ang = math.radians(rotation + i * step)
        r = outer if i % 2 == 0 else inner
        pts.append((cx * K + math.cos(ang) * r * K, cy * K + math.sin(ang) * r * K))
    draw.polygon(pts, fill=color)


def _glow(img, cx, cy, r, color, alpha=90):
    layer = Image.new("RGBA", img.size, (0, 0, 0, 0))
    ImageDraw.Draw(layer).ellipse(_px(cx - r, cy - r, cx + r, cy + r), fill=color + (alpha,))
    img.alpha_composite(layer.filter(ImageFilter.GaussianBlur(radius=r * K * 0.5)))


def draw_icon():
    """按 1024 坐标系绘制，返回超采样后的 RGBA 图像。"""
    img, mask = _tile()
    d = ImageDraw.Draw(img)

    # 基线
    d.rounded_rectangle(_px(232, 812, 792, 848), radius=round(18 * K), fill=(34, 68, 110, 255))
    # 三根蜡烛：空心蓝 → 实心浅蓝 → 实心琥珀（上涨）
    d.rounded_rectangle(_px(236, 640, 362, 800), radius=round(26 * K),
                        outline=(86, 138, 246, 255), width=round(30 * K))
    d.rounded_rectangle(_px(452, 512, 578, 800), radius=round(26 * K), fill=(122, 176, 255, 255))
    d.rounded_rectangle(_px(668, 362, 794, 800), radius=round(26 * K), fill=(250, 176, 60, 255))

    # 火花：光晕必须裁在圆角方块内，否则会在图标外留一圈脏边
    _glow(img, 794, 250, 140, (255, 208, 92), alpha=90)
    img.putalpha(ImageChops.multiply(img.getchannel("A"), mask))
    _sparkle(ImageDraw.Draw(img), 794, 250, 100, 28, (255, 214, 102, 255))
    return img


def main():
    os.makedirs(BUILD, exist_ok=True)
    big = draw_icon()
    master = big.resize((1024, 1024), Image.LANCZOS)

    targets = [
        (os.path.join(BUILD, "appicon.png"), None),
        (os.path.join(BUILD, "windows", "icon.ico"), "ICO"),
        (os.path.join(BUILD, "app.ico"), "ICO"),
    ]
    for path, fmt in targets:
        os.makedirs(os.path.dirname(path), exist_ok=True)
        if fmt == "ICO":
            master.save(path, format="ICO", sizes=[(s, s) for s in ICO_SIZES])
            print("wrote %s (sizes: %s)" % (path, ",".join(str(s) for s in ICO_SIZES)))
        else:
            master.save(path)
            print("wrote %s (%dx%d)" % (path, master.width, master.height))
    return 0


if __name__ == "__main__":
    sys.exit(main())
