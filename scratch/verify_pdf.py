import sys
from pypdf import PdfReader

def verify_pdf(pdf_path):
    print(f"Reading PDF: {pdf_path}")
    reader = PdfReader(pdf_path)
    text = ""
    for page in reader.pages:
        text += page.extract_text()
    
    print("\n--- Extracted Text ---")
    print(text)
    print("----------------------\n")

    services_header = "SERVICES PROVIDED"
    parts_header = "PARTS USED"

    has_services = services_header in text
    has_parts = parts_header in text

    print(f"Contains '{services_header}': {has_services}")
    print(f"Contains '{parts_header}': {has_parts}")

    if has_services and has_parts:
        print("\nSUCCESS: Both services and parts are correctly segmented in the PDF!")
        sys.exit(0)
    else:
        print("\nFAILURE: Missing segmented headers in the PDF!")
        sys.exit(1)

if __name__ == "__main__":
    verify_pdf("d:/erp-microservices/erp-billing-service/scratch/invoice.pdf")
