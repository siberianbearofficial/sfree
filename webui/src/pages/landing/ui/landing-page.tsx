import {Button} from "@heroui/react";

export function LandingPage() {
  return (
    <div className="min-h-screen flex flex-col items-center justify-center gap-4">
      <h1 className="text-4xl font-bold">S3aaS</h1>
      <p>Simple storage as a service.</p>
      <div className="flex gap-4">
        <Button color="primary">Sign Up</Button>
        <Button variant="bordered">Log In</Button>
      </div>
    </div>
  );
}
