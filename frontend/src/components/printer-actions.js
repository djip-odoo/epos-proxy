export async function copyPrinterFieldValue(printer, field, copiedIds) {
    await navigator.clipboard.writeText(printer[field]);
    (copiedIds[printer.id] ||= {})[field] = true;
    setTimeout(() => copiedIds[printer.id][field] = false, 2000);
}

async function sendEposPrint(printerIp, name, openCashDrawer = false) {
  const content = openCashDrawer
    ? '<pulse />'
    : `
        <feed line="1" />
        <text font="font_e" em="true"/>
        <text align="center">This is a test receipt ${name}</text>
        <feed line="3" />
        <cut type="feed" />
      `

  return await fetch(`http://${printerIp}/cgi-bin/epos/service.cgi`, {
    method: 'POST',
    body: `<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/">
          <s:Body>
            <epos-print xmlns="http://www.epson-pos.com/schemas/2011/03/epos-print">
            ${content}
            </epos-print>
          </s:Body>
        </s:Envelope>`
  })
}

export async function executePrint(printer, openCashDrawer) {
  const response = await sendEposPrint(printer.ip, printer.name, openCashDrawer)
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
}
