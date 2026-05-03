#!/usr/bin/env python3
"""Korva demo GIF v2 — higher quality Pillow render at 5fps."""
import json, urllib.request
from PIL import Image, ImageDraw, ImageFont
from pathlib import Path

# ── Config ─────────────────────────────────────────────────────────────────────
W, H    = 1280, 720
FPS     = 5
DELAY   = int(1000 / FPS)  # cs per frame for PIL
OUT     = "/tmp/korva_demo_v2.gif"
FONT    = "/System/Library/Fonts/Menlo.ttc"
SANS    = "/System/Library/Fonts/Helvetica.ttc"

# ── Colors (matches HTML demo) ─────────────────────────────────────────────────
BG      = (6,   6,   8)
SURF    = (13,  17,  23)
SURF2   = (22,  27,  34)
BORDER  = (33,  38,  45)
TEXT    = (230, 237, 243)
MUTED   = (139, 148, 158)
DIM     = (72,  79,  88)
GREEN   = (34,  197, 94)
RED     = (248, 81,  73)
BLUE    = (56,  139, 253)
ORANGE  = (210, 153, 34)
PURPLE  = (163, 113, 247)
CYAN    = (57,  211, 83)
DARK_RED_BG = (30, 10, 10)
DARK_GRN_BG = (8,  22,  12)

# ── Fonts ──────────────────────────────────────────────────────────────────────
try:
    F10 = ImageFont.truetype(FONT, 10)
    F11 = ImageFont.truetype(FONT, 11)
    F12 = ImageFont.truetype(FONT, 12)
    F13 = ImageFont.truetype(FONT, 13)
    F14 = ImageFont.truetype(FONT, 14)
    FHDR= ImageFont.truetype(SANS, 12)
    FBIG= ImageFont.truetype(SANS, 22)
    FMED= ImageFont.truetype(SANS, 14)
    FSML= ImageFont.truetype(SANS, 11)
except Exception:
    F10=F11=F12=F13=F14=FHDR=FBIG=FMED=FSML = ImageFont.load_default()

# ── Live vault data ────────────────────────────────────────────────────────────
def fetch_stats():
    try:
        with urllib.request.urlopen('http://localhost:7437/api/v1/stats', timeout=2) as r:
            return json.loads(r.read())
    except Exception:
        return {"total_observations": 15, "total_sessions": 2, "total_prompts": 2,
                "by_type": {"decision":2,"pattern":2,"bugfix":1,"learning":2,"feature":2,
                            "refactor":1,"discovery":1,"incident":1,"antipattern":1,"context":1,"task":1},
                "by_project": {"korva":9,"payments":6}}

STATS = fetch_stats()

# ── Drawing helpers ────────────────────────────────────────────────────────────
def text(d, xy, s, font, fill, anchor="la"):
    d.text(xy, s, font=font, fill=fill, anchor=anchor)

def rect(d, box, fill=None, outline=None, radius=4):
    d.rounded_rectangle(box, radius=radius, fill=fill, outline=outline)

def hline(d, y, color=BORDER):
    d.line([(0,y),(W,y)], fill=color, width=1)

def vline(d, x, color=BORDER):
    d.line([(x,0),(x,H)], fill=color, width=1)

# ── Header ─────────────────────────────────────────────────────────────────────
def draw_header(d, services_on=None):
    """services_on: set of service names that are lit (green)."""
    if services_on is None:
        services_on = set()
    d.rectangle([(0,0),(W,44)], fill=SURF)
    hline(d, 44)
    # Logo
    rect(d, [16,8,40,36], fill=GREEN, radius=6)
    text(d, (28,22), "K", FMED, (0,0,0), "mm")
    text(d, (48,10), "korva", FHDR, TEXT)
    text(d, (48,26), "The cognitive OS for AI-driven teams", F10, MUTED)
    # Service badges
    svc_colors = {"Vault":BLUE,"Sentinel":ORANGE,"Lore":PURPLE,"Forge":CYAN,"Hive":GREEN,"Beacon":RED}
    x = 220
    for svc, col in svc_colors.items():
        is_on = svc in services_on
        bg    = col if is_on else SURF2
        fg    = (0,0,0) if is_on else DIM
        w     = int(FHDR.getlength(svc)) + 16
        rect(d, [x,11,x+w,33], fill=bg, outline=col if not is_on else None, radius=4)
        text(d, (x+w//2,22), svc, FSML, fg, "mm")
        x += w + 6
    # LIVE badge
    badge = "● LIVE DEMO"
    bw    = int(FHDR.getlength(badge)) + 20
    rect(d, [W-bw-10,10,W-10,34], fill=(0,0,0), outline=GREEN, radius=4)
    text(d, (W-bw//2-10,22), badge, FSML, GREEN, "mm")

# ── Stage bar ──────────────────────────────────────────────────────────────────
def draw_stage(d, label):
    d.rectangle([(0,44),(W,62)], fill=SURF2)
    rect(d, [12,47,20,57], fill=GREEN, radius=2)
    text(d, (25,54), label, FSML, MUTED)
    hline(d, 62)

# ── Split panels ───────────────────────────────────────────────────────────────
MID = W // 2

def draw_panels(d):
    d.rectangle([(0,62),(W,H-42)], fill=BG)
    # Panel titles
    d.rectangle([(0,62),(MID,82)], fill=DARK_RED_BG)
    d.rectangle([(MID,62),(W,82)], fill=DARK_GRN_BG)
    text(d, (14,72), "✗ Sin Korva — AI sin memoria", F12, RED)
    text(d, (MID+14,72), "✓ Con Korva — AI con memoria persistente", F12, GREEN)
    vline(d, MID)
    hline(d, 82)

# ── Footer stats bar ───────────────────────────────────────────────────────────
STATS_DATA = [
    ("OBS",     str(STATS['total_observations'])),
    ("SESIONES", str(STATS['total_sessions'])),
    ("SCROLLS",  "25"),
    ("MCP TOOLS","23"),
    ("TOKEN SAVING","−80%"),
    ("IDEs",    "8"),
]
def draw_footer(d, reveal=False):
    y0 = H - 42
    d.rectangle([(0,y0),(W,H)], fill=SURF)
    hline(d, y0)
    cols = len(STATS_DATA)
    cw   = W // cols
    for i,(label,val) in enumerate(STATS_DATA):
        x  = i*cw + cw//2
        fg = GREEN if reveal and label=="TOKEN SAVING" else (TEXT if reveal else DIM)
        vl = val if reveal else "—"
        text(d, (x,y0+10), vl, FBIG if reveal else FMED, fg, "mt")
        text(d, (x,y0+34), label, F10, MUTED, "mb")
    if reveal:
        # draw a thin green accent on TOKEN SAVING
        ti = 4  # index 4
        x0 = ti*cw; x1=x0+cw
        d.rectangle([(x0,y0),(x1,y0+2)], fill=GREEN)

# ── Line helpers ───────────────────────────────────────────────────────────────
LINE_H = 16
LEFT_X  = 10
RIGHT_X = MID + 10
TOP_Y   = 86

def make_frame(services_on=None, stage="", left_lines=None, right_lines=None,
               footer_reveal=False, sentinel_box=None, token_bars=None):
    """Build one PIL Image frame."""
    img = Image.new("RGB", (W,H), BG)
    d   = ImageDraw.Draw(img)
    draw_header(d, services_on or set())
    draw_stage(d, stage)
    draw_panels(d)
    draw_footer(d, footer_reveal)

    # Left lines
    if left_lines:
        for i, (txt, col) in enumerate(left_lines):
            y = TOP_Y + i*LINE_H
            if y+LINE_H > H-46: break
            text(d, (LEFT_X, y), txt, F12, col)

    # Right lines
    if right_lines:
        for i, (txt, col) in enumerate(right_lines):
            y = TOP_Y + i*LINE_H
            if y+LINE_H > H-46: break
            text(d, (RIGHT_X, y), txt, F12, col)

    # Sentinel error box
    if sentinel_box:
        bx0, by0 = MID+10, sentinel_box['y']
        bw2 = MID - 20
        bh  = len(sentinel_box['lines'])*LINE_H + 14
        rect(d, [bx0,by0,bx0+bw2,by0+bh], fill=(40,8,8), outline=RED, radius=6)
        text(d, (bx0+10, by0+6), f"✗ {sentinel_box['title']}", F12, RED)
        for li, line in enumerate(sentinel_box['lines']):
            text(d, (bx0+10, by0+6+(li+1)*LINE_H), line, F12, (230,100,100))

    # Token savings bars
    if token_bars:
        bx0 = MID + 10
        for bi, bar in enumerate(token_bars):
            y = bar['y'] + bi*42
            text(d, (bx0, y), bar['label'], F12, MUTED)
            bw2 = MID - 20
            # background track
            rect(d, [bx0, y+18, bx0+bw2, y+32], fill=SURF2, radius=3)
            # filled bar
            fill_w = int(bw2 * bar['pct'])
            if fill_w > 0:
                rect(d, [bx0, y+18, bx0+fill_w, y+32], fill=bar['color'], radius=3)
            # label
            text(d, (bx0+bw2+5, y+24), bar['tag'], F11, bar['color'])

    return img

# ── Scene definitions ───────────────────────────────────────────────────────────
# Each scene = (duration_sec, frame_fn_returning_img)
# We'll build a list of frames directly.

def repeat(frames, img, count):
    frames.extend([img]*count)

def build_frames():
    frames = []
    f = FPS  # frames per second

    # ────────────────────────────────── SCENE 0: Intro ──────────────────────────
    # 1.5s — blank terminals
    intro_left = [
        ("usuario: Implementa el checkout. Usa Redis lock", TEXT),
        ("         en payment_id, como la vez anterior.",  TEXT),
        ("AI : // Session #47 — igual que la sesión #1",  RED),
        ("     // No recuerdo esa decisión…",              MUTED),
    ]
    repeat(frames, make_frame(
        stage="Escena 0 — ¿Qué pasa cuando el AI no tiene memoria?",
        left_lines=intro_left,
    ), int(f*1.5))

    # ────────────────────────────────── SCENE 1: El problema ────────────────────
    bad_code = [
        ("usuario: Implementa el checkout. Usa Redis lock", TEXT),
        ("         en payment_id, como la vez anterior.",  TEXT),
        ("AI : // Session #47 — igual que la sesión #1",  RED),
        ("     // No recuerdo esa decisión…",              MUTED),
        ("",                                               TEXT),
        ("async createOrder(req, res) {",                  DIM),
        ("  // ← directo a DB, sin lock distribuido",      DIM),
        ("  const order = await db.query('INSERT ...')",   DIM),
        ("  await sendEmail(order.id)   // ← sync call",   DIM),
        ("}",                                              DIM),
        ("",                                               TEXT),
        ("✗ Race condition — mismo bug del 28-Abr",        RED),
        ("✗ El AI no sabe del outage P1 de $12K",          RED),
        ("✗ Patrón correcto perdido. Dev lo re-explica",   RED),
        ("// Sesión termina. Todo se pierde.",              MUTED),
        ("// Mañana otro dev hace las mismas preguntas.",   MUTED),
        ("",                                               TEXT),
        ("$ git add src/auth/config.ts && git commit",     TEXT),
        ("  -m 'feat: auth'",                              TEXT),
        ("// sk_live_4xK9mP... ahora está en git history", RED),
        ("✗ Brecha de seguridad · rotación de key urgente",RED),
    ]
    repeat(frames, make_frame(
        stage="Escena 1 — El problema: sin memoria persistente",
        left_lines=bad_code,
    ), int(f*3))

    # ────────────────────────────────── SCENE 2: vault_context ──────────────────
    ctx_right = [
        ("$ vault_context ({ project: \"payments\" })",          GREEN),
        ("→ Conectando al vault · localhost:7437",               MUTED),
        ("→ Cargando contexto del proyecto \"payments\"…",        MUTED),
        ("",                                                     TEXT),
        ("┌─ VAULT_CONTEXT → 6 observaciones · 142ms ─────────┐",BLUE),
        ("│ ✓ Redis lock en payment_id · idempotency           │",TEXT),
        ("│ ✓ circuit breaker activo · timeout 30s             │",TEXT),
        ("│ ✓ Race condition bugfix del 28-Abr (evitar)        │",TEXT),
        ("│ ✓ Patrón hexagonal · validar en domain layer       │",TEXT),
        ("│ ✓ Outage P1 $12K — no repetir este patrón          │",TEXT),
        ("└────────────────────────────────────────────────────┘",BLUE),
        ("",                                                     TEXT),
        ("✓ AI sabe: usar Redis lock · idempotency              ",GREEN),
        ("✓ Genera código correcto. Primera vez.                ",GREEN),
    ]
    repeat(frames, make_frame(
        stage="Escena 2 — vault_context: el AI carga contexto persistente",
        services_on={"Vault"},
        left_lines=bad_code,
        right_lines=ctx_right,
    ), int(f*3))

    # ────────────────────────────────── SCENE 3: vault_save ─────────────────────
    save_right = ctx_right + [
        ("",                                                     TEXT),
        ("$ vault_save ({",                                      GREEN),
        ("    type: \"bugfix\",",                                 MUTED),
        ("    title: \"Race condition - Redis lock payment_id\",", MUTED),
        ("    content: \"SETNX TTL 30s. Rollout canary 5%.\"",    MUTED),
        ("})",                                                    GREEN),
        ("✓ Guardado · ID: 01KQMVV1E4…",                        GREEN),
        ("✓ Disponible para todo el equipo · próxima sesión",    GREEN),
        ("✓ Hive: sincronizará el patrón a la comunidad",        BLUE),
    ]
    repeat(frames, make_frame(
        stage="Escena 3 — vault_save: conocimiento persistente para el equipo",
        services_on={"Vault","Hive"},
        left_lines=bad_code,
        right_lines=save_right,
    ), int(f*3))

    # ────────────────────────────────── SCENE 4: Sentinel ───────────────────────
    commit_right = save_right + [
        ("",                                                     TEXT),
        ("$ git commit -m \"feat: user authentication\"",        TEXT),
        ("Running Korva Sentinel…",                              MUTED),
    ]
    sentinel = {
        "y": TOP_Y + len(commit_right)*LINE_H + 8,
        "title": "3 issues críticos — Commit bloqueado",
        "lines": [
            "✗ SEC-001 Hardcoded secret · src/auth/config.ts:14",
            "✗ SEC-003 Timing attack vulnerability · AuthService.ts:31",
            "✗ ARC-002 HTTP handler en domain layer · violación hexagonal",
        ],
    }
    repeat(frames, make_frame(
        stage="Escena 4 — Sentinel: guardrails automáticos en cada commit",
        services_on={"Vault","Sentinel","Hive"},
        left_lines=bad_code,
        right_lines=commit_right,
        sentinel_box=sentinel,
    ), int(f*4))

    # ────────────────────────────────── SCENE 5: Lore + Forge ───────────────────
    lore_right = save_right + [
        ("",                                                     TEXT),
        ("$ vault_lore_context ()",                              PURPLE),
        ("→ 25 Scrolls aplicados · arquitectura hexagonal",      PURPLE),
        ("→ Reglas ARC validadas · NestJS + Fastify + Nx",       PURPLE),
        ("",                                                     TEXT),
        ("$ vault_team_context ()",                              CYAN),
        ("→ Forge SDD phase: design · en progreso",              CYAN),
        ("→ Criterios de calidad activos · Go+TypeScript",       CYAN),
        ("✓ Forge impone revisión humana antes de implementar",  GREEN),
    ]
    repeat(frames, make_frame(
        stage="Escena 5 — Lore: scrolls de conocimiento · Forge: workflow SDD",
        services_on={"Vault","Sentinel","Lore","Forge","Hive"},
        left_lines=bad_code,
        right_lines=lore_right,
    ), int(f*3))

    # ────────────────────────────────── SCENE 6: Token savings ──────────────────
    token_right = lore_right + [
        ("",                                                     TEXT),
        ("$ vault_compress ({ mode: \"lite\" })",                ORANGE),
    ]
    token_bars = [
        {"y": TOP_Y + len(token_right)*LINE_H + 4, "label": "lite:  ~83 tok", "pct": 0.20, "color": ORANGE, "tag": "−80%"},
        {"y": TOP_Y + len(token_right)*LINE_H + 4, "label": "ultra: ~13 tok", "pct": 0.03, "color": GREEN,  "tag": "−97%"},
    ]
    repeat(frames, make_frame(
        stage="Escena 6 — vault_compress: −80% tokens · sesiones más largas",
        services_on={"Vault","Sentinel","Lore","Forge","Hive"},
        left_lines=bad_code,
        right_lines=token_right,
        token_bars=token_bars,
    ), int(f*3))

    # ────────────────────────────────── SCENE 7: Final stats ────────────────────
    ntypes = len(STATS.get("by_type",{}))
    nproj  = len(STATS.get("by_project",{}))
    stats_right = lore_right + [
        ("",                                                     TEXT),
        ("$ vault_stats ()",                                     GREEN),
        (f"✓ {STATS['total_observations']} obs · {ntypes} tipos · {nproj} proyectos",GREEN),
        (f"✓ {STATS['total_sessions']} sesiones · {STATS.get('total_prompts',2)} prompts guardados", GREEN),
        ("✓ vault_compress disponible · modo lite/ultra",        BLUE),
        ("✓ 23 herramientas MCP · 3 perfiles de seguridad",     BLUE),
        ("✓ 8 IDEs soportados · Cursor/Copilot/Claude/Zed…",    PURPLE),
        ("",                                                     TEXT),
        ("✓ VALIDADO — Korva protege cada sesión de tu equipo",  GREEN),
    ]
    repeat(frames, make_frame(
        stage="Escena 7 — Korva: memoria, guardrails y ahorro de tokens",
        services_on={"Vault","Sentinel","Lore","Forge","Hive","Beacon"},
        left_lines=bad_code,
        right_lines=stats_right,
        footer_reveal=True,
    ), int(f*5))

    return frames

# ── Main ────────────────────────────────────────────────────────────────────────
print("Generating frames…")
frames = build_frames()
print(f"  {len(frames)} frames @ {FPS}fps = {len(frames)/FPS:.1f}s")

print("Saving GIF…")
frames[0].save(
    OUT,
    save_all=True,
    append_images=frames[1:],
    duration=DELAY,
    loop=0,
    optimize=False,
)
size = Path(OUT).stat().st_size
print(f"  → {OUT}  ({size/1024:.0f} KB)")
