import {Card, CardHeader, CardBody, CardFooter, Button} from "@heroui/react";
import {Link} from "react-router-dom";

const features = [
  {title: "Buckets", description: "Manage storage buckets", href: "/buckets"},
  {title: "Sources", description: "Configure data sources", href: "/sources"},
];

export function DashboardPage() {
  return (
    <div className="p-8 flex flex-col gap-8">
      <h1 className="text-3xl font-bold">Dashboard</h1>
      <div className="grid gap-6 sm:grid-cols-2">
        {features.map((feature) => (
          <Card key={feature.title}>
            <CardHeader>{feature.title}</CardHeader>
            <CardBody>{feature.description}</CardBody>
            <CardFooter>
              <Button as={Link} color="primary" to={feature.href}>
                Open
              </Button>
            </CardFooter>
          </Card>
        ))}
      </div>
    </div>
  );
}
