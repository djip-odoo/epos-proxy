
import test_pdf_file from "../assets/pdf/test_pdf.pdf"

export async function copyPrinterFieldValue(printer, field = 'ip', { copiedIds, showToast }) {
  try {
    await navigator.clipboard.writeText(printer[field])
    if (!copiedIds.value[printer.id]) {
      copiedIds.value[printer.id] = {}
    }
    copiedIds.value[printer.id][field] = true
    setTimeout(() => copiedIds.value[printer.id][field] = false, 2000)
  } catch (err) {
    showToast('Copy failed:' + err)
  }
}

async function sendPdf(printerIp) {
  const res = await fetch(test_pdf_file)
  const blob = await res.blob()

  return await fetch(`http://${printerIp}/print/pdf`, {
    method: "POST",
    headers: {
      "Content-Type": "application/pdf",
    },
    body: blob,
    signal: AbortSignal.timeout(60000),
  })
}

async function sendEposPrint(printerIp, name) {
  return await fetch(`http://${printerIp}/cgi-bin/epos/service.cgi`, {
    method: 'POST',
    signal: AbortSignal.timeout(60000),
    body: `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
          <s:Body>
            <epos-print xmlns="http://www.epson-pos.com/schemas/2011/03/epos-print">
              <feed line="1" />
              <text font="font_e" em="true"/>
              <text align="center">This is a test receipt ${name}</text>
              <feed line="10" />
              <cut type="feed" />
            </epos-print>
          </s:Body>
        </s:Envelope>`
  })
}

export async function handleTestPrint(printer, { testPrintIds, selectedPrinter, showTypeSelect, showToast }) {
  if (printer.type === 'ANY') {
    selectedPrinter.value = printer
    showTypeSelect.value = true
    return
  }

  testPrintIds.value[printer.id] = true
  try {
    return await executePrint(printer, showToast)
  } finally {
    testPrintIds.value[printer.id] = false
  }
}

async function executePrint(printer, showToast) {
  try {
    if (printer.type === 'EPOS') {
      const response = await sendEposPrint(printer.ip, printer.name)
      const xml = await response.text()
      const parser = new DOMParser()
      const doc = parser.parseFromString(xml, 'text/xml')
      const responseEl = doc.querySelector('response')

      if (responseEl?.getAttribute('success') !== 'true') {
        const code = responseEl?.getAttribute('code') || 'Unknown error'
        if (code === 'EX_BADPORT') {
          throw new Error('The device is not connected, please check the printer power / connection')
        }
        throw new Error(code)
      }

      showToast(`Test print sent`, 'success')

    } else {
      const response = await sendPdf(printer.ip);
      if (!response.ok) throw new Error('Network response was not ok')
      showToast(`Test print sent to ${printer.name}`, 'success')
    }
  } catch (err) {
    showToast(`Test failed: ${err.message}`, 'error')
  }
}
