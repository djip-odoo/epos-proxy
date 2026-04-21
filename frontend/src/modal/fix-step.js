import image1 from "../assets/zadig/zadig1.jpg";
import image2 from "../assets/zadig/zadig2.jpg";
import image3 from "../assets/zadig/zadig3.jpg";
import cups_admin from "../assets/cups/cups_admin.png";

export function zadigSteps(filter) {
    return filter === "THERMAL"
        ? [
              {
                  title: "Download Zadig",
                  desc: "Download the latest version of Zadig from the official website.",
                  link: "https://zadig.akeo.ie",
                  linkLabel: "Open Zadig website",
              },
              {
                  title: "Open Zadig",
                  desc: "Run Zadig.exe as Administrator.\nGo to Options → List All Devices.",
                  image: image1,
              },
              {
                  title: "Select your printer",
                  desc: `Find the printer in the dropdown list.\n The name may vary, but it should look similar to printer.`,
                  image: image2,
              },
              {
                  title: "Install WinUSB driver",
                  desc: 'Make sure WinUSB is selected, then click "Replace Driver". Wait for completion.',
                  image: image3,
              },
          ]
        : [];
}

export function brewSteps(filter) {
    return filter === "THERMAL"
        ? [
              {
                  title: "Install Homebrew",
                  desc: "Homebrew is a package manager for macOS. If you already have it, skip to the next step.",
                  link: "https://brew.sh",
                  linkLabel: "Open Homebrew website",
              },
              {
                  title: "Install libusb",
                  desc: "Once Homebrew is installed, run this command in Terminal:",
                  codes: ["brew install libusb"],
              },
              {
                  title: "Reconnect your printer",
                  desc: `Unplug and replug your printer.\nThe driver should now be detected automatically.`,
              },
          ]
        : [];
}

export function linuxSteps(filter) {
    return filter === "THERMAL"
        ? [
              {
                  title: "Install libusb",
                  desc: "Open a terminal and run the command for your distribution:",
                  codes: [
                      "# Debian / Ubuntu\nsudo apt-get install libusb-1.0-0",
                      "# Fedora / RHEL\nsudo dnf install libusb1",
                      "# Arch\nsudo pacman -S libusb",
                  ],
              },
              {
                  title: "Assign permissions to your user",
                  desc: "Open a terminal and run the command for your distribution:",
                  codes: [
                      "# Debian / Ubuntu\nsudo usermod -aG lp,plugdev $USER",
                      "# Fedora / RHEL\nsudo usermod -aG lp,dialout $USER",
                      "# Arch\nsudo usermod -aG lp,uucp $USER",
                  ],
              },
              {
                  title: "Reconnect your printer and apply changes",
                  desc: `Unplug and reconnect your printer. Then restart your device to apply the new group permissions.`,
              },
          ]
        : [
              {
                  title: "Open CUPS Admin Panel",
                  desc: "Open your browser and go to *http://localhost:631*. This is the CUPS web interface.",
                  link: "http://localhost:631",
                  linkLabel: "Open CUPS Admin Panel",
              },
              {
                  title: "Go to Administration",
                  desc: "Click on the *Administration* tab in the CUPS panel. You may be asked to enter your system *username* and *password*.",
                  image: cups_admin,
              },
              {
                  title: "Add Printer",
                  desc: "Click on *Add Printer*",
              },
              {
                  title: "Select Printer",
                  desc: "Choose your printer from the *Local Printer* list and click *Continue*.",
              },
              {
                  title: "Configure Printer",
                  desc: "Set printer *name*, *description*, and *location*. then click *Continue*.",
              },
              {
                  title: "Choose Driver",
                  desc: "Select the appropriate *driver* from the list *or* provide a *PPD file* if required.",
              },
              {
                  title: "Finish Setup",
                  desc: "Click *Add Printer*",
              },
          ];
}
