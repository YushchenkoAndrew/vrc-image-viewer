import { exec } from "child_process";
import { readdirSync } from "fs";
import { NextRequest, NextResponse } from "next/server";
import { join } from "path";

const PATH =
  "/home/mortis/.steam/debian-installation/steamapps/compatdata/438100/pfx/drive_c/users/steamuser/Pictures/VRChat";

export async function GET(request: NextRequest) {
  const dirs = readdirSync(PATH);
  const files = [] as string[];
  for (const folder of dirs) {
    for (const file of readdirSync(join(PATH, folder))) {
      exec(`ln -sf ${join(PATH, folder, file)} public/${file}`);
      files.push(`/${file}`);
    }
  }

  return NextResponse.json({ files: files.sort().reverse() });
}
