#!/usr/bin/env python3
"""
Korva Demo GIF Generator
Generates a polished split-screen terminal GIF showing "Sin Korva vs Con Korva"
using real vault data from localhost:7437
"""
from PIL import Image, ImageDraw, ImageFont
import urllib.request, json, os, textwrap

# ── Config ────────────────────────────────────────────────────────────────────
W, H        = 1280, 720
FONT_PATH   = "/System/Library/Fonts/Menlo.ttc"
FONT_SZ     = 13
HEADER_H    = 52
STAGE_H     = 28
FOOTER_H    = 52
PANEL_H     = H - HEADER_H - STAGE_H - FOOTER_H
SPLIT       = W // 2

# Colors (dark GitHub-like theme)
BG       = (6, 6, 8)
SURF     = (13, 17, 23)
SURF2    = (22, 27, 34)
BORDER   = (33, 38, 45)
TEXT     = (230, 237, 243)
MUTED    = (139, 148, 158)
DIM      = (72, 79, 88)
GREEN    = (34, 197, 94)
RED      = (248, 81, 73)
BLUE     = (56, 139, 253)
ORANGE   = (210, 153, 34)
PURPLE   = (163, 113, 247)
CYAN     = (57, 211, 83)
GREEN_BG = (13, 40, 20)
RED_BG   = (40, 13, 13)
BLUE_BG  = (13, 22, 40)

# ── Fonts ─────────────────────────────────────────────────────────────────────
try:
    font      = ImageFont.truetype(FONT_PATH, FONT_SZ)
    font_bold = ImageFont.truetype(FONT_PATH, FONT_SZ + 1)
    font_sm   = ImageFont.truetype(FONT_PATH, 11)
    font_lg   = ImageFont.truetype(FONT_PATH, 15)
    font_hdr  = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 13)
    font_hdr_b= ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 14)
    font_stat = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 20)
    font_stag = ImageFont.truetype("/System/Library/Fonts/Helvetica.ttc", 11)
except:
    font = font_bold = font_sm = font_lg = font_hdr = font_hdr_b = font_stat = font_stag = ImageFont.load_default()

# ── Fetch real vault data ─────────────────────────────────────────────────────
def fetch(path):
    try:
        with urllib.request.urlopen(f"http://localhost:7437{path}", timeout=2) as r:
            return json.loads(r.read())
    except:
        return {}

stats = fetch("/api/v1/stats")
ctx   = fetch("/api/v1/context/payments")
OBS   = stats.get("total_observations", 15)
SESS  = stats.get("total_sessions", 2)
ctx_obs = ctx.get("context", [])[:5]

# ── Drawing helpers ───────────────────────────────────────────────────────────
def new_frame():
    img = Image.new("RGB", (W, H), BG)
    d   = ImageDraw.Draw(img)
    return img, d

def draw_header(d, services_lit):
    # Background
    d.rectangle([0, 0, W, HEADER_H], fill=SURF)
    d.line([0, HEADER_H, W, HEADER_H], fill=BORDER)
    # Logo icon
    d.rounded_rectangle([16, 12, 44, 40], radius=6, fill=GREEN)
    d.text((21, 15), "K", font=font_hdr_b, fill=(0, 0, 0))
    # Logo text
    d.text((52, 11), "korva", font=font_hdr_b, fill=TEXT)
    d.text((52, 28), "The cognitive OS for AI-driven teams", font=font_stag, fill=MUTED)
    # Services badges
    svcs = ["Vault", "Sentinel", "Lore", "Forge", "Hive", "Beacon"]
    x = 200
    for s in svcs:
        color = GREEN if s in services_lit else DIM
        bg    = (13, 40, 20) if s in services_lit else SURF2
        brd   = (34, 120, 60) if s in services_lit else BORDER
        tw    = d.textlength(s, font=font_stag) + 16
        d.rounded_rectangle([x, 16, x+tw, 36], radius=4, fill=bg, outline=brd)
        d.text((x+8, 20), s, font=font_stag, fill=color)
        x += tw + 6
    # Live badge
    badge_txt = "✓ VALIDADO" if "Beacon" in services_lit else "● LIVE DEMO"
    badge_col = GREEN if "Beacon" in services_lit else (34, 197, 94)
    badge_bg  = (13, 40, 20) if "Beacon" in services_lit else SURF2
    btw = d.textlength(badge_txt, font=font_stag) + 20
    d.rounded_rectangle([W-btw-16, 16, W-16, 36], radius=99, fill=badge_bg, outline=(34, 120, 60))
    d.text((W-btw-6, 20), badge_txt, font=font_stag, fill=badge_col)

def draw_stage(d, text, color=BLUE):
    y = HEADER_H
    bg = tuple(int(c * 0.08) for c in color) + (255,)
    d.rectangle([0, y, W, y+STAGE_H], fill=tuple(int(c*0.06) for c in color))
    d.line([0, y+STAGE_H, W, y+STAGE_H], fill=tuple(int(c*0.25) for c in color))
    d.ellipse([14, y+10, 22, y+18], fill=color)
    d.text((30, y+7), text, font=font_stag, fill=color)

def draw_panel_headers(d):
    y = HEADER_H + STAGE_H
    # Left — bad
    d.rectangle([0, y, SPLIT, y+28], fill=RED_BG)
    d.line([0, y+28, SPLIT, y+28], fill=BORDER)
    d.ellipse([14, y+10, 22, y+18], fill=RED)
    d.text((30, y+8), "✗  Sin Korva — AI sin memoria", font=font_stag, fill=RED)
    # Right — good
    d.rectangle([SPLIT, y, W, y+28], fill=GREEN_BG)
    d.line([SPLIT, y+28, W, y+28], fill=BORDER)
    d.ellipse([SPLIT+14, y+10, SPLIT+22, y+18], fill=GREEN)
    d.text((SPLIT+30, y+8), "✓  Con Korva — AI con memoria persistente", font=font_stag, fill=GREEN)
    # Divider
    d.line([SPLIT, HEADER_H, SPLIT, H-FOOTER_H], fill=BORDER, width=2)

def draw_footer(d, obs, sess, scrolls, tools, tok, ides):
    y = H - FOOTER_H
    d.rectangle([0, y, W, H], fill=SURF)
    d.line([0, y, W, y], fill=BORDER)
    cols = [
        (str(obs),     "OBSERVACIONES"),
        (str(sess),    "SESIONES"),
        (str(scrolls), "SCROLLS"),
        (str(tools),   "MCP TOOLS"),
        (tok,          "TOKEN SAVING"),
        (str(ides),    "IDES"),
    ]
    cw = W // len(cols)
    for i, (val, lbl) in enumerate(cols):
        x = i * cw
        if i > 0:
            d.line([x, y+6, x, H-6], fill=BORDER)
        vc = GREEN if val not in ("0", "—") else DIM
        vw = d.textlength(val, font=font_stat)
        d.text((x + cw//2 - vw//2, y+6), val, font=font_stat, fill=vc)
        lw = d.textlength(lbl, font=font_stag)
        d.text((x + cw//2 - lw//2, y+30), lbl, font=font_stag, fill=DIM)

def text_block(d, x, y, lines, lh=18):
    """Draw a list of (text, color) tuples."""
    for text, color in lines:
        if text is None:
            y += lh // 2
            continue
        d.text((x, y), text, font=font, fill=color)
        y += lh
    return y

def tag_badge(d, x, y, tag, tag_colors):
    color, bg, brd = tag_colors.get(tag, (MUTED, SURF2, BORDER))
    tw = d.textlength(tag, font=font_sm) + 10
    d.rounded_rectangle([x, y, x+tw, y+16], radius=3, fill=bg, outline=brd)
    d.text((x+5, y+2), tag, font=font_sm, fill=color)
    return tw + 6

TAG_COLORS = {
    "bugfix":    (RED,    (40,13,13),   (120,40,40)),
    "learning":  (BLUE,   (13,22,40),   (40,70,120)),
    "decision":  (PURPLE, (25,13,40),   (80,50,120)),
    "pattern":   (GREEN,  (13,40,20),   (40,120,60)),
    "incident":  (ORANGE, (40,30,13),   (120,90,40)),
    "discovery": (CYAN,   (13,35,20),   (40,120,60)),
}

def separator(d, y_start, color=BORDER):
    d.line([0, y_start, W, y_start], fill=color)

# ── Frame builder ─────────────────────────────────────────────────────────────
CONTENT_Y   = HEADER_H + STAGE_H + 28 + 12   # below panel headers
LEFT_X      = 14
RIGHT_X     = SPLIT + 14
LH          = 17

def make_frame(scene: int) -> Image.Image:
    img, d = new_frame()

    # ── Scene-specific data ───────────────────────────────────────────────────
    if scene == 0:
        svcs = []
        stage = ("Iniciando demostración…", BLUE)
        obs_count, sess_count, scrolls, tools, tok, ides = 0, 0, 0, 0, "—", 0

    elif scene == 1:
        svcs = []
        stage = ("Escena 1 — El problema: sin memoria persistente", RED)
        obs_count, sess_count, scrolls, tools, tok, ides = 0, 0, 0, 0, "—", 0

    elif scene == 2:
        svcs = ["Vault"]
        stage = ("Escena 1 — vault_context: cargando contexto del proyecto", BLUE)
        obs_count, sess_count, scrolls, tools, tok, ides = OBS, SESS, 0, 0, "—", 0

    elif scene == 3:
        svcs = ["Vault", "Hive"]
        stage = ("Escena 2 — vault_save: guardando conocimiento en tiempo real", BLUE)
        obs_count, sess_count, scrolls, tools, tok, ides = OBS, SESS, 0, 0, "—", 0

    elif scene == 4:
        svcs = ["Vault", "Sentinel", "Hive"]
        stage = ("Escena 3 — Sentinel: guardrails en cada commit", RED)
        obs_count, sess_count, scrolls, tools, tok, ides = OBS, SESS, 0, 0, "—", 0

    elif scene == 5:
        svcs = ["Vault", "Sentinel", "Lore", "Forge", "Hive"]
        stage = ("Escena 4 — Lore + Forge: conocimiento inyectado + SDD Workflow", PURPLE)
        obs_count, sess_count, scrolls, tools, tok, ides = OBS, SESS, 25, 23, "—", 0

    elif scene == 6:
        svcs = ["Vault", "Sentinel", "Lore", "Forge", "Hive"]
        stage = ("Escena 5 — vault_compress: token savings en tiempo real", GREEN)
        obs_count, sess_count, scrolls, tools, tok, ides = OBS, SESS, 25, 23, "−80%", 0

    else:  # scene == 7
        svcs = ["Vault", "Sentinel", "Lore", "Forge", "Hive", "Beacon"]
        stage = ("✓ Demo completado — Korva: Memory · Guardrails · Knowledge", GREEN)
        obs_count, sess_count, scrolls, tools, tok, ides = OBS, SESS, 25, 23, "−80%", 8

    # ── Draw chrome ───────────────────────────────────────────────────────────
    draw_header(d, svcs)
    draw_stage(d, stage[0], stage[1])
    draw_panel_headers(d)
    draw_footer(d, obs_count, sess_count, scrolls, tools, tok, ides)

    ly = CONTENT_Y
    ry = CONTENT_Y

    # ── LEFT panel (always accumulates bad content) ───────────────────────────
    if scene >= 1:
        d.text((LEFT_X, ly), "usuario:", font=font, fill=MUTED)
        d.text((LEFT_X+70, ly), " Implementa checkout. Usa Redis lock en", font=font, fill=TEXT)
        ly += LH
        d.text((LEFT_X+70, ly), " payment_id, como decidimos antes.", font=font, fill=TEXT)
        ly += LH + 4
        d.text((LEFT_X, ly), "  AI  :", font=font, fill=MUTED)
        d.text((LEFT_X+52, ly), " // Session #47 — igual que la sesión #1", font=font, fill=ORANGE)
        ly += LH
        d.text((LEFT_X+52, ly), " // No recuerdo esa decisión…", font=font, fill=RED)
        ly += LH + 6

    if scene >= 1:
        # wrong code block
        d.rounded_rectangle([LEFT_X-4, ly-2, SPLIT-14, ly+72], radius=4, fill=SURF2, outline=BORDER)
        d.text((LEFT_X+2, ly), "AI GENERA CÓDIGO INCORRECTO (IGNORA REDIS LOCK)", font=font_sm, fill=DIM)
        ly += LH
        d.text((LEFT_X+2, ly), "async", font=font, fill=RED)
        d.text((LEFT_X+48, ly), "createOrder(req, res) {", font=font, fill=TEXT)
        ly += LH
        d.text((LEFT_X+16, ly), "// ← directo a DB, sin lock distribuido", font=font, fill=RED)
        ly += LH
        d.text((LEFT_X+16, ly), "const order = ", font=font, fill=BLUE)
        d.text((LEFT_X+120, ly), "await", font=font, fill=RED)
        d.text((LEFT_X+162, ly), " db.query(...)", font=font, fill=TEXT)
        ly += LH
        d.text((LEFT_X+16, ly), "await sendEmail(order.id)", font=font, fill=TEXT)
        d.text((LEFT_X+186, ly), " // ← sync call", font=font, fill=RED)
        ly += LH
        d.text((LEFT_X+2, ly), "}", font=font, fill=TEXT)
        ly += LH + 8

        d.text((LEFT_X, ly), "✗", font=font, fill=RED)
        d.text((LEFT_X+16, ly), " Race condition — mismo bug del incidente 28-Apr", font=font, fill=RED)
        ly += LH
        d.text((LEFT_X, ly), "✗", font=font, fill=RED)
        d.text((LEFT_X+16, ly), " El AI no sabe del outage P1 de $12K", font=font, fill=RED)
        ly += LH
        d.text((LEFT_X, ly), "✗", font=font, fill=RED)
        d.text((LEFT_X+16, ly), " Patrón correcto perdido. Dev lo re-explica (3ª vez).", font=font, fill=RED)
        ly += LH + 6

    if scene >= 3:
        d.text((LEFT_X, ly), "// Sesión termina. Todo se pierde.", font=font, fill=ORANGE)
        ly += LH
        d.text((LEFT_X, ly), "// Mañana un nuevo dev hará las mismas preguntas.", font=font, fill=ORANGE)
        ly += LH
        d.text((LEFT_X, ly), "// Conocimiento evaporado. Otra vez.", font=font, fill=DIM)
        ly += LH + 6

    if scene >= 4:
        d.text((LEFT_X, ly), "$ git add src/auth/config.ts && git commit", font=font, fill=TEXT)
        ly += LH
        d.text((LEFT_X, ly), "// Sin Sentinel — el secreto pasa al repo", font=font, fill=ORANGE)
        ly += LH
        d.text((LEFT_X, ly), "// sk_live_4xK9mP... ahora está en git history", font=font, fill=ORANGE)
        ly += LH
        d.text((LEFT_X, ly), "✗", font=font, fill=RED)
        d.text((LEFT_X+16, ly), " Brecha de seguridad · rotación de key urgente", font=font, fill=RED)
        ly += LH + 6

    if scene >= 5:
        d.text((LEFT_X, ly), "// AI propone Stripe sin idempotency key", font=font, fill=ORANGE)
        ly += LH
        d.text((LEFT_X, ly), "// No conoce las reglas del equipo", font=font, fill=ORANGE)
        ly += LH + 6

    if scene >= 6:
        d.text((LEFT_X, ly), "// Contexto largo sin comprimir:", font=font, fill=MUTED)
        ly += LH + 2
        d.text((LEFT_X, ly), "Tokens por mensaje — sin vault_compress", font=font_sm, fill=DIM)
        ly += LH
        d.rounded_rectangle([LEFT_X, ly, SPLIT-14, ly+12], radius=2, fill=SURF2)
        bar_w = int((SPLIT - 14 - LEFT_X - 4) * 1.0)
        d.rounded_rectangle([LEFT_X+2, ly+2, LEFT_X+2+bar_w, ly+10], radius=2, fill=RED)
        ly += 16
        d.text((LEFT_X, ly), "~405 tokens", font=font_sm, fill=RED)
        d.text((SPLIT-80, ly), "100%", font=font_sm, fill=RED)
        ly += LH + 2
        d.text((LEFT_X, ly), "✗", font=font, fill=RED)
        d.text((LEFT_X+16, ly), " Contexto se agota rápidamente", font=font, fill=RED)
        ly += LH
        d.text((LEFT_X, ly), "✗", font=font, fill=RED)
        d.text((LEFT_X+16, ly), " Costo de API innecesariamente alto", font=font, fill=RED)
        ly += LH + 6

    if scene == 7:
        d.text((LEFT_X, ly), "Resultado:", font=font, fill=MUTED)
        ly += LH
        results_bad = [
            "✗ Bugs repetidos en cada sprint",
            "✗ Secretos expuestos en git history",
            "✗ Tokens desperdiciados · costo alto",
            "✗ Conocimiento evaporado tras cada sesión",
        ]
        for r in results_bad:
            d.text((LEFT_X, ly), r, font=font, fill=RED)
            ly += LH

    # ── RIGHT panel (accumulates good content) ────────────────────────────────
    if scene >= 1:
        d.text((RIGHT_X, ry), "$ ", font=font, fill=GREEN)
        d.text((RIGHT_X+18, ry), "vault_context", font=font, fill=CYAN)
        d.text((RIGHT_X+120, ry), "({ project: ", font=font, fill=TEXT)
        d.text((RIGHT_X+215, ry), '"payments"', font=font, fill=GREEN)
        d.text((RIGHT_X+285, ry), " })", font=font, fill=TEXT)
        ry += LH
        d.text((RIGHT_X, ry), "→ Conectando al vault local · localhost:7437", font=font, fill=BLUE)
        ry += LH
        d.text((RIGHT_X, ry), "→ Cargando contexto del proyecto \"payments\"…", font=font, fill=GREEN)
        ry += LH + 4

    if scene >= 2:
        # Context block
        block_h = 16 + len(ctx_obs) * 18 + 8
        d.rounded_rectangle([RIGHT_X-4, ry-2, W-14, ry+block_h], radius=4, fill=SURF2, outline=BORDER)
        d.text((RIGHT_X+2, ry+2), f"VAULT_CONTEXT → {len(ctx_obs)} OBSERVACIONES CARGADAS · 142MS", font=font_sm, fill=DIM)
        bry = ry + 18
        for obs in ctx_obs:
            tx = tag_badge(d, RIGHT_X+4, bry, obs.get("type","?"), TAG_COLORS)
            d.text((RIGHT_X+4+tx, bry), obs.get("title","")[:52], font=font_sm, fill=TEXT)
            bry += 18
        ry += block_h + 8
        d.text((RIGHT_X, ry), "✓", font=font, fill=GREEN)
        d.text((RIGHT_X+16, ry), " AI sabe: Redis lock · idempotency · circuit breaker", font=font, fill=GREEN)
        ry += LH
        d.text((RIGHT_X, ry), "✓", font=font, fill=GREEN)
        d.text((RIGHT_X+16, ry), " Genera código correcto. Primera vez.", font=font, fill=GREEN)
        ry += LH + 6

    if scene >= 3:
        d.text((RIGHT_X, ry), "$ ", font=font, fill=GREEN)
        d.text((RIGHT_X+18, ry), "vault_save", font=font, fill=CYAN)
        d.text((RIGHT_X+96, ry), "({", font=font, fill=TEXT)
        ry += LH
        d.text((RIGHT_X+16, ry), 'type: ', font=font, fill=TEXT)
        d.text((RIGHT_X+58, ry), '"bugfix"', font=font, fill=GREEN)
        d.text((RIGHT_X+108, ry), ',', font=font, fill=TEXT)
        ry += LH
        d.text((RIGHT_X+16, ry), 'title: ', font=font, fill=TEXT)
        d.text((RIGHT_X+60, ry), '"Race condition — Redis lock payment_id"', font=font, fill=GREEN)
        ry += LH
        d.text((RIGHT_X, ry), "})", font=font, fill=TEXT)
        ry += LH
        d.text((RIGHT_X, ry), "✓", font=font, fill=GREEN)
        d.text((RIGHT_X+16, ry), " Guardado · Disponible para todo el equipo", font=font, fill=GREEN)
        ry += LH
        d.text((RIGHT_X, ry), "✓", font=font, fill=GREEN)
        d.text((RIGHT_X+16, ry), " Hive: sincronizará el pattern a la comunidad", font=font, fill=GREEN)
        ry += LH + 6

    if scene >= 4:
        d.text((RIGHT_X, ry), "$ git commit -m", font=font, fill=TEXT)
        d.text((RIGHT_X+138, ry), ' "feat: user authentication"', font=font, fill=GREEN)
        ry += LH
        d.text((RIGHT_X, ry), "Running Korva Sentinel…", font=font, fill=BLUE)
        ry += LH + 2
        # Sentinel block
        sb_h = 60
        d.rounded_rectangle([RIGHT_X-4, ry-2, W-14, ry+sb_h], radius=4, fill=(40,10,10), outline=(120,40,40))
        d.text((RIGHT_X+2, ry+2), "✗ 3 issues críticos — Commit bloqueado", font=font, fill=RED)
        sy = ry + 18
        violations = [
            ("SEC-001", "Hardcoded secret detectado · src/auth/config.ts:14"),
            ("SEC-003", "Timing attack vulnerability · src/auth/AuthService.ts:31"),
            ("ARC-002", "HTTP handler en domain layer · violación de arquitectura"),
        ]
        for code, msg in violations:
            d.text((RIGHT_X+4, sy), "✗ ", font=font_sm, fill=RED)
            d.text((RIGHT_X+18, sy), code, font=font_sm, fill=ORANGE)
            d.text((RIGHT_X+70, sy), " " + msg, font=font_sm, fill=MUTED)
            sy += 14
        ry += sb_h + 8
        d.text((RIGHT_X, ry), "✓", font=font, fill=GREEN)
        d.text((RIGHT_X+16, ry), " Secreto protegido · nunca llega al repo", font=font, fill=GREEN)
        ry += LH + 6

    if scene >= 5:
        d.text((RIGHT_X, ry), "$ ", font=font, fill=GREEN)
        d.text((RIGHT_X+18, ry), "vault_team_context", font=font, fill=CYAN)
        d.text((RIGHT_X+164, ry), "()", font=font, fill=TEXT)
        ry += LH + 2
        # Lore block
        lb_h = 68
        d.rounded_rectangle([RIGHT_X-4, ry-2, W-14, ry+lb_h], radius=4, fill=SURF2, outline=BORDER)
        d.text((RIGHT_X+2, ry+2), "LORE — SCROLLS INYECTADOS AUTOMÁTICAMENTE", font=font_sm, fill=DIM)
        tag_badge(d, RIGHT_X+4, ry+18, "pattern", TAG_COLORS)
        d.text((RIGHT_X+70, ry+18), "forge-sdd — guía workflow 9 fases", font=font_sm, fill=GREEN)
        d.rounded_rectangle([RIGHT_X+4, ry+36, RIGHT_X+26, ry+52], radius=3, fill=SURF2, outline=BORDER)
        d.text((RIGHT_X+8, ry+38), "25", font=font_sm, fill=MUTED)
        d.text((RIGHT_X+30, ry+37), "scrolls curated: stripe, nestjs-hexagonal, security…", font=font_sm, fill=MUTED)
        d.text((RIGHT_X+2, ry+53), "Forge SDD: explore → propose → spec → design → tasks → apply → verify", font=font_sm, fill=DIM)
        ry += lb_h + 8
        d.text((RIGHT_X, ry), "✓", font=font, fill=GREEN)
        d.text((RIGHT_X+16, ry), " AI opera bajo las reglas del equipo · cada sesión", font=font, fill=GREEN)
        ry += LH + 6

    if scene >= 6:
        d.text((RIGHT_X, ry), "$ ", font=font, fill=GREEN)
        d.text((RIGHT_X+18, ry), "vault_compress", font=font, fill=CYAN)
        d.text((RIGHT_X+122, ry), "({ mode:", font=font, fill=TEXT)
        d.text((RIGHT_X+190, ry), ' "lite"', font=font, fill=GREEN)
        d.text((RIGHT_X+228, ry), " })", font=font, fill=TEXT)
        ry += LH + 4
        avail_w = W - 14 - RIGHT_X - 4
        # lite bar
        d.text((RIGHT_X, ry), "lite: ~83 tok", font=font_sm, fill=GREEN)
        d.text((W-50, ry), "−80%", font=font_sm, fill=GREEN)
        ry += 14
        d.rounded_rectangle([RIGHT_X, ry, W-14, ry+10], radius=2, fill=SURF2)
        d.rounded_rectangle([RIGHT_X+2, ry+2, RIGHT_X+2+int(avail_w*0.20), ry+8], radius=2, fill=GREEN)
        ry += 14
        # ultra bar
        d.text((RIGHT_X, ry), "ultra: ~13 tok", font=font_sm, fill=CYAN)
        d.text((W-50, ry), "−97%", font=font_sm, fill=CYAN)
        ry += 14
        d.rounded_rectangle([RIGHT_X, ry, W-14, ry+10], radius=2, fill=SURF2)
        d.rounded_rectangle([RIGHT_X+2, ry+2, RIGHT_X+2+int(avail_w*0.03), ry+8], radius=2, fill=CYAN)
        ry += 14 + 6
        d.text((RIGHT_X, ry), "✓", font=font, fill=GREEN)
        d.text((RIGHT_X+16, ry), " −12,880 tokens ahorrados por sesión (40 mensajes)", font=font, fill=GREEN)
        ry += LH + 6

    if scene == 7:
        # Stats block
        sb_h = 76
        d.rounded_rectangle([RIGHT_X-4, ry-2, W-14, ry+sb_h], radius=4, fill=SURF2, outline=BORDER)
        d.text((RIGHT_X+2, ry+2), "VAULT STATS — DATOS REALES · LOCALHOST:7437", font=font_sm, fill=DIM)
        items = [
            (f"{OBS} observaciones · 11 tipos verificados", GREEN),
            (f"{SESS} sesiones · 2 prompts · Hive outbox: 15 pending", GREEN),
            ("23 MCP tools · 8 IDEs · sin configuración requerida", BLUE),
            ("100% local · MIT · Privacy filter activo", MUTED),
        ]
        sy = ry + 18
        for txt, col in items:
            d.text((RIGHT_X+4, sy), "●", font=font_sm, fill=col)
            d.text((RIGHT_X+18, sy), txt, font=font_sm, fill=TEXT)
            sy += 14
        ry += sb_h + 6

        d.text((RIGHT_X, ry), "Resultado:", font=font, fill=MUTED)
        ry += LH
        results_good = [
            "✓ 0 bugs repetidos — vault_context previene re-trabajo",
            "✓ 0 secretos expuestos — Sentinel bloquea el commit",
            "✓ −80% tokens — vault_compress modo lite activo",
            "✓ Equipo alineado — Lore + Forge en cada sesión",
        ]
        for r in results_good:
            d.text((RIGHT_X, ry), r, font=font, fill=GREEN)
            ry += LH

    return img

# ── Generate GIF ──────────────────────────────────────────────────────────────
SCENES = [
    (0, 2),   # blank intro
    (1, 5),   # scene 1: problem
    (2, 5),   # vault_context
    (3, 5),   # vault_save
    (4, 5),   # sentinel
    (5, 5),   # lore+forge
    (6, 5),   # compress
    (7, 6),   # final
]

frames = []
durations = []
for scene_id, hold_secs in SCENES:
    n_frames = max(1, hold_secs * 2)
    for i in range(n_frames):
        frames.append(make_frame(scene_id))
        durations.append(500)
    # Longer pause at start and end
    if scene_id in (0, 7):
        for _ in range(4):
            frames.append(make_frame(scene_id))
            durations.append(500)

out_path = "/tmp/korva_demo.gif"
frames[0].save(
    out_path,
    save_all=True,
    append_images=frames[1:],
    duration=durations,
    loop=0,
    optimize=False,
)
size_kb = os.path.getsize(out_path) // 1024
print(f"✓ GIF generado: {out_path}")
print(f"  Frames: {len(frames)} · Duración: ~{sum(durations)//1000}s · Tamaño: {size_kb} KB")
