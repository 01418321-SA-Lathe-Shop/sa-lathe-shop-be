"""Create item-specific private PDF fixtures for the local lathe-shop dataset."""

import os
from pathlib import Path
from xml.sax.saxutils import escape

from reportlab import rl_config
from reportlab.lib import colors
from reportlab.lib.pagesizes import A4
from reportlab.lib.styles import ParagraphStyle
from reportlab.lib.units import mm
from reportlab.pdfbase import pdfmetrics
from reportlab.pdfbase.ttfonts import TTFont
from reportlab.platypus import SimpleDocTemplate, Paragraph, Spacer, Table, TableStyle

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "internal" / "demoworkflow" / "assets"
OUT.mkdir(parents=True, exist_ok=True)
rl_config.invariant = 1
FONT = os.environ.get("LATHE_SEED_THAI_FONT", "/Users/pradipat/Library/Fonts/NotoSansThai-VariableFont_wdth,wght.ttf")
if not Path(FONT).is_file():
    raise SystemExit("Set LATHE_SEED_THAI_FONT to a local Noto Sans Thai TTF file")
pdfmetrics.registerFont(TTFont("ThaiSeed", FONT))
INK = colors.HexColor("#24211e")
ORANGE = colors.HexColor("#e97519")
MUTED = colors.HexColor("#726d68")
LINE = colors.HexColor("#ded8d2")
PAPER = colors.HexColor("#fffaf6")
BASE = ParagraphStyle("base", fontName="ThaiSeed", fontSize=9.2, leading=15, textColor=INK)
TITLE = ParagraphStyle("title", parent=BASE, fontSize=18, leading=25, spaceAfter=5)
SUB = ParagraphStyle("sub", parent=BASE, fontSize=8, leading=12, textColor=MUTED)
HEAD = ParagraphStyle("head", parent=BASE, fontSize=10, leading=15, spaceBefore=11, spaceAfter=4)
SMALL = ParagraphStyle("small", parent=BASE, fontSize=8, leading=12)


def para(text, style=BASE):
    return Paragraph(escape(str(text)), style)


def build(key, title, number, fields, lines, note):
    path = OUT / f"{key}.pdf"
    doc = SimpleDocTemplate(str(path), pagesize=A4, leftMargin=20 * mm,
                            rightMargin=20 * mm, topMargin=23 * mm, bottomMargin=20 * mm,
                            title=f"{title} {number}", author="Lathe Shop Control")
    def page(canvas, document):
        canvas.saveState()
        w, h = A4
        canvas.setFillColor(ORANGE)
        canvas.roundRect(20 * mm, h - 15 * mm, 8 * mm, 8 * mm, 2 * mm, fill=1, stroke=0)
        canvas.setFillColor(INK)
        canvas.setFont("ThaiSeed", 10)
        canvas.drawString(31 * mm, h - 12 * mm, "LATHE SHOP CONTROL")
        canvas.setStrokeColor(LINE)
        canvas.line(20 * mm, 17 * mm, w - 20 * mm, 17 * mm)
        canvas.setFont("ThaiSeed", 7)
        canvas.setFillColor(MUTED)
        canvas.drawString(20 * mm, 11 * mm, "ข้อมูลสมมติเพื่อทดสอบระบบ - ไม่มีการซื้อขายหรือชำระเงินจริง")
        canvas.drawRightString(w - 20 * mm, 11 * mm, f"หน้า {document.page}")
        canvas.restoreState()
    story = [para(title, TITLE), para(number, SUB), Spacer(1, 8 * mm)]
    rows = [[para(label, SMALL), para(value, BASE)] for label, value in fields]
    table = Table(rows, colWidths=[43 * mm, 127 * mm], hAlign="LEFT")
    table.setStyle(TableStyle([
        ("VALIGN", (0, 0), (-1, -1), "TOP"),
        ("BACKGROUND", (0, 0), (0, -1), PAPER),
        ("LINEBELOW", (0, 0), (-1, -2), 0.4, LINE),
        ("LEFTPADDING", (0, 0), (-1, -1), 8),
        ("RIGHTPADDING", (0, 0), (-1, -1), 8),
        ("TOPPADDING", (0, 0), (-1, -1), 7),
        ("BOTTOMPADDING", (0, 0), (-1, -1), 7),
    ]))
    story += [table, para("รายละเอียดรายการ", HEAD)]
    for line in lines:
        story.append(para("• " + line))
    story += [Spacer(1, 6 * mm), para("หมายเหตุ", HEAD), para(note)]
    doc.build(story, onFirstPage=page, onLaterPages=page)
    return path


SCENARIOS = [
    ("r1", "ปลอกบูชเพลาเครื่องผสม S45C", "S45C Ø30 x 45 มม.", 2, "2,400.00", "คุณสมชาย ใจดี", "somchai"),
    ("r2", "ฐานแม่พิมพ์อัดยางรุ่น CM-120", "เหล็ก SS400 120 x 80 x 30 มม.", 3, "6,400.00", "บริษัท สยาม พรีซิชั่น พาร์ท จำกัด", "company"),
    ("r3", "เพลทแม่พิมพ์ยางรุ่น RP-80", "เหล็ก S45C 80 x 60 x 12 มม.", 4, "2,800.00", "บริษัท สยาม พรีซิชั่น พาร์ท จำกัด", "company"),
    ("r4", "ชุดแม่พิมพ์ยางขนาด 160 มม.", "เหล็ก S45C Ø160 x 40 มม.", 5, "18,000.00", "บริษัท สยาม พรีซิชั่น พาร์ท จำกัด", "company"),
    ("r5", "เพลาขับเครื่องลำเลียง Ø25", "เหล็ก S45C Ø25 x 320 มม.", 6, "2,400.00", "คุณสมชาย ใจดี", "somchai"),
    ("r6", "อินเสิร์ตแม่พิมพ์ SUS304", "สแตนเลส SUS304 45 x 35 x 20 มม.", 7, "8,000.00", "บริษัท สยาม พรีซิชั่น พาร์ท จำกัด", "company"),
    ("r7", "ปลอกบูชเครื่องกด S45C", "เหล็ก S45C Ø40 x 30 มม.", 8, "6,400.00", "บริษัท สยาม พรีซิชั่น พาร์ท จำกัด", "company"),
    ("r8", "พินยึดแม่พิมพ์ยาง", "เหล็ก S45C Ø12 x 70 มม.", 9, "4,800.00", "คุณสมชาย ใจดี", "somchai"),
    ("r9", "เพลทจับชิ้นงานรุ่น FP-90", "อะลูมิเนียม AL6061 90 x 70 x 15 มม.", 10, "9,600.00", "บริษัท สยาม พรีซิชั่น พาร์ท จำกัด", "company"),
    ("r10", "บล็อกแม่พิมพ์ NBR-45", "เหล็ก S45C 100 x 80 x 35 มม.", 11, "7,200.00", "คุณสมชาย ใจดี", "somchai"),
    ("r11", "ปลอกบูชบรอนซ์เครื่องบรรจุ", "บรอนซ์ Ø35 x 40 มม.", 12, "6,400.00", "บริษัท สยาม พรีซิชั่น พาร์ท จำกัด", "company"),
]

made = []
for key, title, spec, qty, price, customer, category in SCENARIOS:
    num = int(key[1:])
    ref = f"REQ-2026-{num:03d}"
    made.append(build(f"{key}-drawing", "เอกสารรายละเอียดงานผลิต", f"SPEC-2026-{num:03d} · อ้างอิง {ref}",
        [("ผู้ว่าจ้าง", customer), ("ชื่องาน", title), ("จำนวน", f"{qty} ชิ้น"), ("วัสดุ / ขนาด", spec), ("ฉบับ", "A · 3 พฤศจิกายน 2569")],
        ["กลึงและเก็บผิวตามขนาดที่ระบุ", "ค่าคลาดเคลื่อนทั่วไป ±0.10 มม.", "ลบคมทุกขอบและตรวจขนาดก่อนส่งมอบ"],
        "ใช้สำหรับประเมินและติดตามงานในระบบตัวอย่าง ขนาดเฉพาะจุดให้ยืนยันจากแบบวิศวกรรมก่อนผลิตจริง"))
    if key in {"r4", "r6", "r7", "r9", "r11"}:
        made.append(build(f"{key}-po-document", "ใบสั่งซื้อของลูกค้า", f"PO-2026-{num:03d} · อ้างอิง {ref}",
            [("ผู้สั่งซื้อ", customer), ("ผู้ขาย", "Lathe Shop Control"), ("รายการ", title), ("จำนวน", f"{qty} ชิ้น"), ("ราคาสุทธิ", f"{price} บาท"), ("เงื่อนไข", "ราคายังไม่รวม VAT; ชำระตามใบเสนอราคา")],
            [f"{title} - {spec} - {qty} ชิ้น", f"ยอดรวม {price} บาท"],
            "ยืนยันการสั่งผลิตตามรายละเอียดและราคาที่ระบุในใบเสนอราคา เอกสารนี้เป็นข้อมูลจำลอง"))
    if key in {"r5", "r8", "r10"}:
        deposit = {"r5": "1,200.00", "r8": "2,400.00", "r10": "3,600.00"}[key]
        made.append(build(f"{key}-deposit-document", "หลักฐานแจ้งชำระเงินมัดจำ", f"DEP-2026-{num:03d} · อ้างอิง {ref}",
            [("ผู้แจ้งชำระ", customer), ("วัตถุประสงค์", f"มัดจำงาน {title}"), ("ยอดแจ้งชำระ", f"{deposit} บาท"), ("วิธี", "โอนผ่านธนาคาร"), ("สถานะ", "รอเจ้าหน้าที่ตรวจสอบรายการ")],
            [f"ยอดตามใบเสนอราคา {price} บาท", f"ยอดมัดจำที่แจ้ง {deposit} บาท"],
            "ข้อมูลจำลองเพื่อทดสอบการตรวจหลักฐาน ไม่ใช่สลิปธนาคารหรือหลักฐานการโอนเงินจริง"))

made.append(build("r10-balance-proof", "หลักฐานแจ้งชำระยอดคงเหลือ", "PAY-2026-010 · อ้างอิง REQ-2026-010",
    [("ผู้แจ้งชำระ", "คุณสมชาย ใจดี"), ("รายการ", "บล็อกแม่พิมพ์ NBR-45"), ("ยอดคงเหลือ", "3,600.00 บาท"), ("วิธี", "โอนผ่านธนาคาร"), ("สถานะ", "รอผู้ดูแลตรวจสอบ")],
    ["ยอดขาย 7,200.00 บาท", "หักมัดจำ 3,600.00 บาท", "ยอดแจ้งชำระงวดสุดท้าย 3,600.00 บาท"],
    "ข้อมูลจำลองเพื่อทดสอบการตรวจหลักฐาน ไม่ใช่สลิปธนาคารหรือหลักฐานการโอนเงินจริง"))

assert len(made) == 20, len(made)
print(f"Generated {len(made)} item-specific PDFs in {OUT}")
