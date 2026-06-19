import WindowsIPv4Setting from "../assets/network/windows/ipv4_setting.png";
import WindowsNetworkSetting from "../assets/network/windows/network_setting.png";

import LinuxIPv4Setting from "../assets/network/linux/ipv4_setting.png";
import LinuxNetworkSetting from "../assets/network/linux/network_setting.png";

import DarwinIPv4Setting from "../assets/network/darwin/ipv4_setting.png";
import DarwinNetworkSetting from "../assets/network/darwin/network_setting.png";

const commonSteps = [
    {
        title: "What is a Network Printer?",
        desc: "A network printer connects to your *Wi-Fi* or *router*. To add it, you need the printer's *IP address*.",
    },
    {
        title: "Do You Already Have the Printer IP Address?",
        desc: "After connecting the printer to your *Wi-Fi* or *router*, some printers automatically print a receipt showing their *IP address*.",
        actions: [
            { label: "Yes", goto: "reserve_ip_address" },
            { label: "No", next: true },
        ]
    },
    {
        title: "Print Self-test Print",
        desc: "Turn off the printer. Hold the *Feed* button while turning it on. The printer should print a page containing its network information.",
    },
    {
        title: "Was an IP Address Printed?",
        desc: "Check the printed page. If you can see an *IP address*, the printer is already connected to the network.",
        actions: [
            { label: "Yes", goto: "reserve_ip_address" },
            { label: "No", next: true },
        ]
    },
    {
        title: "Can Your Computer Reach the Printer?",
        desc: "Check whether the printer's IP address looks similar to the addresses used on your network. If you are unsure, select *No*",
        actions: [
            { label: "Yes", goto: "reserve_ip_address" },
            { label: "No", next: true },
        ]
    },
    {
        title: "Connect the Printer Directly",
        desc: "Connect the printer directly to your computer using an Ethernet cable. Make sure the network lights are on.",
    },
];

const osSpecificSteps = {
    windows: [
        {
            title: "Change Your Network Settings",
            desc: "If the printer page does not open, temporarily change your Ethernet settings so your computer can communicate with the printer.",
            image: WindowsNetworkSetting,
        },
        {
            title: "Enter These Settings",
            desc: "Use the following example settings if the printer's IP address shown on the self-test print is *192.168.192.168*. If your printer shows a different IP address, adjust the values accordingly.",
            image: WindowsIPv4Setting,
            codes: [
                `IP Address: 192.168.192.169
Subnet Mask: 255.255.255.0
Default Gateway: Leave blank
Preferred DNS: 1.1.1.1`,
            ],
        },
    ],

    linux: [
        {
            title: "Change Your Network Settings",
            desc: "Open *Settings → Network → Wired Settings → IPv4* and switch from *Automatic (DHCP)* to *Manual*. Configure an IP address in the same range as the printer.",
            image: LinuxNetworkSetting,
        },
        {
            title: "Enter These Settings",
            desc: "Use the following example settings if the printer's IP address shown on the self-test print is *192.168.192.168*. If your printer shows a different IP address, adjust the values accordingly.",
            image: LinuxIPv4Setting,
            codes: [
                `Address: 192.168.192.170
Netmask: 255.255.255.0
Gateway: Leave blank`,
            ],
        },
    ],

    darwin: [
        {
            title: "Change Your Network Settings",
            desc: "Open *System Preferences → Network → Ethernet → Configure IPv4* and switch from *Automatic (DHCP)* to *Manual*. Configure an IP address in the same range as the printer.",
            image: DarwinNetworkSetting,
        },
        {
            title: "Enter These Settings",
            desc: "Use the following example settings if the printer's IP address shown on the self-test print is *192.168.192.168*. If your printer shows a different IP address, adjust the values accordingly.",
            image: DarwinIPv4Setting,
            codes: [
                `IP Address: 192.168.192.170
Subnet Mask: 255.255.255.0
Default Gateway: Leave blank`,
            ],
        },
    ],
};

const finalSteps = [
    {
        title: "Open the Printer Configuration Page",
        desc: "Open a web browser and navigate to the printer's IP address.",
        codes: ["http://192.168.192.168"],
    },
    {
        title: "Configure Printer Network Settings",
        desc: "Open the printer's network settings.\n\nassign a *static(Manual)* IP address that matches your router's network range. Also subnet mask and gatway as per your router.",
        codes: [
            `e.g.:
IP Address: 192.168.0.101
Subnet Mask: 255.255.255.0
Gateway: 192.168.0.1`,
        ],
    },
    {
        id: "reserve_ip_address",
        title: "Keep the Same IP Address",
        desc: "Open your router settings and reserve the printer's current IP address using its *MAC address*. This ensures the printer always receives the same IP address and remains easy to find on your network.",
    },
    {
        title: "Add the Printer",
        desc: "Click *+ Add Network Printer*, enter the printer IP address, and click *Add*.",
    },
    {
        title: "Check Connection Status",
        desc: "*Green*: Online\n*Orange*: Checking status\n*Red*: Unreachable",
    },
    {
        title: "Important Note If using printer with ethernet cable to router",
        desc: "For reliable operation, keep an always-on device (like computer) connected to the same wired Ethernet network as the printer. Some network setups may cause the printer to become unreachable after being idle for a short time if no active device like computer in network.",
    },
];

export default function getNetworkPrinterSteps(os) {
    return [
        ...commonSteps,
        ...osSpecificSteps[os],
        ...finalSteps,
    ];
}
