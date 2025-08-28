import {Button, useDisclosure} from "@heroui/react";
import {LoginDialog, RegisterDialog} from "../../../features/auth";

export function LandingPage() {
  const login = useDisclosure();
  const register = useDisclosure();
  return (
    <div className="min-h-screen flex flex-col items-center justify-center gap-4">
      <h1 className="text-4xl font-bold">S3aaS</h1>
      <p>Simple storage as a service.</p>
      <div className="flex gap-4">
        <Button color="primary" onPress={register.onOpen}>Sign Up</Button>
        <Button variant="bordered" onPress={login.onOpen}>Log In</Button>
      </div>
      <RegisterDialog isOpen={register.isOpen} onOpenChange={register.onOpenChange} />
      <LoginDialog isOpen={login.isOpen} onOpenChange={login.onOpenChange} />
    </div>
  );
}
