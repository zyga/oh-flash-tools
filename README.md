# OH Flashing Tools

Tools for flashing Open Harmony builds on evaluation devices.

## Preparation and Usage

The tool is in an early stages of development and assumes a specific
environment. You need to have the Hi3518ev300 development kit. You also need a
Bus Pirate which acts as a controllable power supply unit for the camera.

## Setting up the Bus Pirate

Connect the Bus Pirate to a host computer. It should enumerate as an FTDI serial
port using USB VID:PID of 0403:6001 and must be the only connected USB device
using that pair.

Consult the bus pirate pin-out diagram and identify the GND and 5V pins. Connect
them to the Hi3518ev300 boards's USB header. Connect only the GND and 5V lines,
leaving the two data pins unused. You can use a JST SH plug (1 row, 4 pins, 1mm
pitch, about 5mm total width) for a mechanically robust connection. If you don't
have that you can use one of the standard Bus Pirate wire straps to connect to
those pins. You may need to unscrew the camera from the plexiglass base in order
for the Bus Pirate cables to fit.

## Setting up Hi3518ev300

Assuming you've set up the Bus Pirate as instructed above you will only need to
connect the second USB to serial adapter. Use the Prolific adapter that comes
with the development kit. It should enumerate using USB VID:PID of 067b:2303 and
again, must be the only connected USB device using that pair.

## Preparing the operating system

Linux distributions should detect the USB serial adapters automatically.
Windows, assuming you are outside of corporate firewall, can do that as well. If
you need to you can grab USB drivers for the two devices from:

1. https://www.ftdichip.com/Drivers/VCP.htm
2. http://www.prolific.com.tw/US/ShowProduct.aspx?p_id=225&pcid=41

## Flashing

Put the following files in the current working directory and run the `oh-flash` tool.

1. `u-boot-hi3518ev300.bin`
2. `OHOS_Image.bin`
3. `rootfs.img`
4. `userfs.img`

You can obtain those binaries from the OHOS build tree, in the `out/` directory,
except for the u-boot binary which is deeper in the tree. Use `find` to locate
it.
