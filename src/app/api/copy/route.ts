import { exec } from "child_process";
import { NextRequest, NextResponse } from "next/server";

export async function POST(request: NextRequest) {
  const body = await request.json();
  exec(`echo "${body.text}" | xclip -selection clipboard`);
  return NextResponse.json({});
}
