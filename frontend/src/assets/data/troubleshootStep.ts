import type { main } from "../../../wailsjs/go/models";

export function windowsSteps(info: main.TroubleshootInfo) {
  return [
    {
      title: "Windows Firewall Rule",
      desc: "If Windows Defender Firewall is blocking other devices on your local network, open *PowerShell as Administrator* and run the following command. \n\nIf the command does not work, you can create the same rule from \n *Windows Defender Firewall → Advanced settings → Inbound Rules → New Rule/Edit Rule (if already exist)*.",
      codes: [
        `New-NetFirewallRule -DisplayName "ePOS Proxy" -Direction Inbound \`\n  -Program "${info.execPath}" \`\n  -Action Allow -Profile Private`,
      ],
    },
    ...networkSteps(info),
  ];
}

export function macSteps(info: main.TroubleshootInfo) {
  return [
    {
      title: "macOS Application Firewall",
      desc: "macOS uses an application firewall. You can allow ePOS Proxy in *System Settings → Privacy & Security → Firewall*, or run this in Terminal:",
      codes: [
        `sudo /usr/libexec/ApplicationFirewall/socketfilterfw --unblockapp "/Applications/ePOS Proxy.app"`,
      ],
    },
    ...networkSteps(info),
  ];
}

export function linuxFirewalldSteps(info: main.TroubleshootInfo) {
  const { port, firewallZone } = info;
  return [
    {
      title: "Allow Port in Firewalld",
      desc: `*firewalld* is active. Run the following commands in your terminal to allow port *${port}* in the *${firewallZone}* zone:`,
      codes: [
        `sudo firewall-cmd --permanent --zone=${firewallZone} --add-port=${port}/tcp\nsudo firewall-cmd --reload`,
      ],
    },
    ...networkSteps(info),
  ];
}

export function linuxUfwSteps(info: main.TroubleshootInfo) {
  const { port, subnet } = info;

  return [
    {
      title: "Allow Port in UFW",
      desc: `*ufw* is active on your system. Run this command in your terminal to allow printing from your local network (*${subnet}*):`,
      codes: [
        `sudo ufw allow from ${subnet} to any port ${port} proto tcp`,
      ],
    },
    ...networkSteps(info),
  ];
}

export function linuxNftablesSteps(info: main.TroubleshootInfo) {
  const { port, subnet } = info;
  return [
    {
      title: "Allow Port in nftables",
      desc: `*nftables* is active. Add a rule to allow incoming TCP traffic on port *${info.port}* from your local network (*${info.subnet}*).\n\n⚠️ This rule is not persistent across reboots. Save it to your nftables config (e.g. \`/etc/nftables.conf\`) to make it permanent.`,
      codes: [
        `sudo nft add rule inet filter input ip saddr ${subnet} tcp dport ${port} accept`,
      ],
    },
    ...networkSteps(info),
  ];
}

export function linuxNoFirewallSteps(info: main.TroubleshootInfo) {
  return [
    {
      title: "Linux Firewall Rule",
      desc: `Install any firewall package like ufw, firewalld or nftables and allow port ${info.port} for incoming connections. Then open this dialog again.`,
    },
    ...networkSteps(info),
  ];
}

function networkSteps({ localIp, port }: main.TroubleshootInfo) {
  return [
    {
      title: "Check Network & Wi-Fi Connection",
      desc: `1. *Same Local Wi-Fi:* Ensure your POS tablet or device is connected to the same Wi-Fi network (not a Guest network or cellular data).\n\n2. *Router Client Isolation:* Check if your Wi-Fi router has "Client Isolation" or "AP Isolation" enabled. This prevents devices from communicating with each other.\n\n3. *Proxy Server Address:* Your proxy server is listening at *http://${localIp}:${port}*`,
    },
    {
      title: "Set a Fixed / Static IP",
      desc: `To ensure your POS connection never breaks when your router reboots or assigns new IP addresses:\n\n *Reserve a fixed / static IP* for this computer (*${localIp}*) in your router's DHCP settings.`,
    },
  ];
}

export function getTroubleshootSteps(info: main.TroubleshootInfo) {
  if (info.os === "windows") {
    return windowsSteps(info);
  }
  if (info.os === "darwin") {
    return macSteps(info);
  }
  if (info.os === "linux") {
    if (info.activeFirewall === "firewalld") {
      return linuxFirewalldSteps(info);
    }
    if (info.activeFirewall === "ufw") {
      return linuxUfwSteps(info);
    }
    if (info.activeFirewall === "nftables") {
      return linuxNftablesSteps(info);
    }
    return linuxNoFirewallSteps(info);
  }
  return networkSteps(info);
}
