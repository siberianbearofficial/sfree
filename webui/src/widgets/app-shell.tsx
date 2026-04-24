import {
  Navbar,
  NavbarBrand,
  NavbarContent,
  NavbarItem,
  NavbarMenuToggle,
  NavbarMenu,
  NavbarMenuItem,
  Button,
  Dropdown,
  DropdownTrigger,
  DropdownMenu,
  DropdownItem,
  Avatar,
} from "@heroui/react";
import {useState} from "react";
import {Link, NavLink, Outlet, useNavigate} from "react-router-dom";
import {logout} from "../shared/lib/auth";

const navItems = [
  {label: "Dashboard", to: "/dashboard"},
  {label: "Sources", to: "/sources"},
  {label: "Buckets", to: "/buckets"},
] as const;

function getUsername(): string {
  return localStorage.getItem("username") ?? "User";
}

function UserInitial({name}: {name: string}) {
  return (
    <Avatar
      size="sm"
      name={name.charAt(0).toUpperCase()}
      classNames={{base: "bg-primary text-primary-foreground cursor-pointer"}}
    />
  );
}

export function AppShell() {
  const [menuOpen, setMenuOpen] = useState(false);
  const navigate = useNavigate();
  const username = getUsername();

  function handleLogout() {
    logout();
    navigate("/");
  }

  return (
    <div className="min-h-screen flex flex-col">
      <Navbar
        maxWidth="full"
        isMenuOpen={menuOpen}
        onMenuOpenChange={setMenuOpen}
        classNames={{
          base: "border-b border-divider",
          wrapper: "px-4 sm:px-6",
        }}
      >
        <NavbarContent className="gap-4" justify="start">
          <NavbarMenuToggle
            aria-label={menuOpen ? "Close menu" : "Open menu"}
            className="sm:hidden"
          />
          <NavbarBrand>
            <Link to="/dashboard" className="font-bold text-lg text-foreground">
              SFree
            </Link>
          </NavbarBrand>
        </NavbarContent>

        <NavbarContent className="hidden sm:flex gap-6" justify="center">
          {navItems.map((item) => (
            <NavbarItem key={item.to}>
              <NavLink
                to={item.to}
                className={({isActive}) =>
                  isActive
                    ? "text-primary font-semibold"
                    : "text-foreground hover:text-primary transition-colors"
                }
              >
                {item.label}
              </NavLink>
            </NavbarItem>
          ))}
        </NavbarContent>

        <NavbarContent justify="end">
          <NavbarItem>
            <Dropdown placement="bottom-end">
              <DropdownTrigger>
                <button type="button" aria-label="User menu">
                  <UserInitial name={username} />
                </button>
              </DropdownTrigger>
              <DropdownMenu aria-label="User actions">
                <DropdownItem key="profile" isReadOnly className="opacity-100">
                  <span className="font-semibold">{username}</span>
                </DropdownItem>
                <DropdownItem
                  key="logout"
                  color="danger"
                  onPress={handleLogout}
                >
                  Log Out
                </DropdownItem>
              </DropdownMenu>
            </Dropdown>
          </NavbarItem>
        </NavbarContent>

        {/* Mobile menu */}
        <NavbarMenu>
          {navItems.map((item) => (
            <NavbarMenuItem key={item.to}>
              <NavLink
                to={item.to}
                className={({isActive}) =>
                  `w-full ${isActive ? "text-primary font-semibold" : "text-foreground"}`
                }
                onClick={() => setMenuOpen(false)}
              >
                {item.label}
              </NavLink>
            </NavbarMenuItem>
          ))}
          <NavbarMenuItem>
            <Button
              variant="flat"
              color="danger"
              className="w-full mt-4"
              onPress={handleLogout}
            >
              Log Out
            </Button>
          </NavbarMenuItem>
        </NavbarMenu>
      </Navbar>

      <main className="flex-1">
        <Outlet />
      </main>
    </div>
  );
}
