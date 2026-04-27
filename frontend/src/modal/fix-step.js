import image1 from "../assets/zadig/zadig1.jpg";
import image2 from "../assets/zadig/zadig2.jpg";
import image3 from "../assets/zadig/zadig3.jpg";
import cups_admin from "../assets/cups/cups_admin.png";
import select_printer from "../assets/cups/select_printer.png";
import select_driver from "../assets/cups/select_driver.png";
import name_desc_loc from "../assets/cups/name_desc_loc.png";

const CUPS_STEPS = [
    {
        title: "Ensure CUPS is Running",
        desc: "*CUPS* must be installed and running before proceeding.",
        codes: [
            "# Install CUPS (if not installed)\n# Debian / Ubuntu\nsudo apt install cups\n# Fedora\nsudo dnf install cups\n# Arch\nsudo pacman -S cups",
            "# Check status\nsudo systemctl status cups || systemctl status cups.service",
            "# Start & enable (if not running)\nsudo systemctl start cups\nsudo systemctl enable cups",
        ],
    },
    {
        title: "Open CUPS Admin Panel",
        desc: "Open your browser and go to *http://localhost:631*.",
        link: "http://localhost:631",
        linkLabel: "Open CUPS Admin Panel",
    },
    {
        title: "Go to Administration",
        desc: "Click on the *Administration* tab. You will be prompted for your system *username* and *password* (admin access required).",
        image: cups_admin,
        warning: "Use an account with *administrator rights*.",
    },
    {
        title: "Add Printer",
        desc: "Click *Add Printer*.",
    },
    {
        title: "Select Printer",
        desc: "Select your printer from the *Local Printers* list and click *Continue*.",
        image: select_printer,
    },
    {
        title: "Set Printer Details",
        desc: "Enter a clear *Name* (no spaces), *Description*, and *Location*, then click *Continue*.",
        image: name_desc_loc,
    },
    {
        title: "Select Driver",
        desc: "Select your *printer model*. If not listed, choose the *closest match* or provide a *PPD file*.",
        image: select_driver,
    },
    {
        title: "Finish & Test",
        desc: "Click *Add Printer* and print a *test page* to verify.",
    },
];

const windowsOfficeSteps = [
    {
        title: "Open Printer Settings",
        desc: "Press *Win + I* → *Bluetooth & devices* → *Printers & scanners*.",
    },
    {
        title: "Add Printer",
        desc: "Click *Add device* (or *Add a printer or scanner*).",
    },
    {
        title: "Select Your Printer",
        desc: "Wait for Windows to detect your printer and follow the *on-screen instructions*.\n\nIf it does not appear after a few seconds, click *Add manually*.",
    },
    {
        title: "Finish",
        desc: "Complete the setup using the Windows wizard and print a *test page* to verify.",
    }
];

const windowsThermalSteps = [
    {
        title: "Download Zadig",
        desc: "Download the latest version from the *official site*.",
        link: "https://zadig.akeo.ie",
        linkLabel: "Download Zadig",
    },
    {
        title: "Run Zadig as Administrator",
        desc: "Right-click *Zadig.exe* → *Run as administrator*.\nGo to *Options → List All Devices*.",
        image: image1,
        warning: "*Administrator access* is required.",
    },
    {
        title: "Select Your Printer",
        desc: "Find your thermal printer in the list (name may vary). Make sure you select the correct *USB device*.",
        image: image2,
        warning: "Double-check you're not selecting another device (e.g., webcam).",
    },
    {
        title: "Install WinUSB Driver",
        desc: "Select *WinUSB* from the driver dropdown.\nClick *Replace Driver* and wait for completion.",
        image: image3,
    },
];

export function macSteps() {
    return {
        OFFICE: CUPS_STEPS,
        THERMAL: [
            {
                title: "Install Homebrew (if not installed)",
                desc: "Homebrew is required for libusb.",
                link: "https://brew.sh",
                linkLabel: "Install Homebrew",
            },
            {
                title: "Install required USB support",
                desc: "Open *Terminal* and copy-paste the command below:",
                codes: ["brew install libusb"],
            },
            {
                title: "Reconnect Printer",
                desc: "Unplug the thermal printer completely (USB cable), wait 5 seconds, then plug it back in.",
            },
        ],
    };
}

export function linuxSteps() {
    return {
        OFFICE: CUPS_STEPS,
        THERMAL: [
            {
                title: "Install required USB support",
                desc: "Open *Terminal* and copy-paste the command below:",
                codes: [
                    "# Debian / Ubuntu\nsudo apt update && sudo apt install libusb-1.0-0",
                    "# Fedora / RHEL\nsudo dnf install libusb1",
                    "# Arch Linux\nsudo pacman -S libusb",
                ],
            },
            {
                title: "Add User to Required Groups",
                desc: "Open *Terminal* and copy-paste the command below:",
                codes: [
                    "# Debian / Ubuntu\nsudo usermod -aG lp,plugdev $USER",
                    "# Fedora / RHEL\nsudo usermod -aG lp,dialout $USER",
                    "# Arch Linux\nsudo usermod -aG lp,uucp $USER",
                ],
            },
            {
                title: "Apply Changes",
                desc: "Restart your system to apply the new group *permissions*.",
            },
            {
                title: "Reconnect",
                desc: "Unplug and reconnect the printer.",
            }
        ],
    };
}

export function windowsSteps() {
    return {
        THERMAL: windowsThermalSteps,
        OFFICE: windowsOfficeSteps,
    };
}
