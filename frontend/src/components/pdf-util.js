import { PDFDocument, StandardFonts, rgb } from "pdf-lib";

const fontSize = 14;
const margin = 50;
const lineHeight = fontSize + 5;
const pageHeight = 841.89;
const pageWidth = 595.28;

export async function createPdfBytes(text, duplex) {
  const pdfDoc = await PDFDocument.create();
  const font = await pdfDoc.embedFont(StandardFonts.Courier);

  const lines = wrapText(text, 500, font, fontSize);
  const maxLinesPage1 = duplex === "duplex" ? 1 : 3;

  const chunks = [
    lines.slice(0, maxLinesPage1),
    lines.slice(maxLinesPage1),
  ].filter(chunk => chunk.length);

  for (const chunk of chunks) {
    drawPage(pdfDoc, font, chunk);
  }

  return await pdfDoc.save();
}

function drawPage(pdfDoc, font, lines) {
  const page = pdfDoc.addPage([pageWidth, pageHeight]);
  let y = pageHeight - margin;

  for (const line of lines) {
    page.drawText(line, {
      x: margin,
      y,
      size: fontSize,
      font,
      color: rgb(0, 0, 0),
    });
    y -= lineHeight;
  }
}

function wrapText(text, maxWidth, font, fontSize) {
  const words = text.split(" ");
  const lines = [];
  let currentLine = "";

  for (const word of words) {
    const testLine = currentLine ? currentLine + " " + word : word;
    const width = font.widthOfTextAtSize(testLine, fontSize);

    if (width < maxWidth) {
      currentLine = testLine;
    } else {
      if (currentLine) lines.push(currentLine);
      currentLine = word;
    }
  }

  if (currentLine) lines.push(currentLine);
  return lines;
}
